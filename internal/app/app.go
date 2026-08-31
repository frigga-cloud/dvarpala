package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"dvarpala/internal/api/v1"
	"dvarpala/internal/api/vpnapi"
	"dvarpala/internal/auth"
	"dvarpala/internal/config"
	"dvarpala/internal/database"
	"dvarpala/internal/redis"
	"dvarpala/internal/services"
	"dvarpala/internal/vpn"
	"dvarpala/internal/web"

	"github.com/gin-gonic/gin"
)

type Dvarpala struct {
	// signIn describes how people can authenticate, kept so it can be
	// reported at the end of startup as well as when it is decided. Gin
	// prints a line per route between the two, and the decision that matters
	// most was scrolling out of sight.
	signIn []string

	sweeper  *vpn.IdleSweeper
	config   *config.Config
	db       *database.DB
	redis    *redis.Client
	services *services.Services
	auth     *auth.Service
	router   *gin.Engine
}

func NewDvarpala(cfg *config.Config) (*Dvarpala, error) {
	// Initialize database
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		return nil, err
	}

	// Initialize database tables and essential data
	if err := database.InitializeDatabase(db.DB); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize Redis
	redisClient, err := redis.NewClient(cfg.Redis)
	if err != nil {
		return nil, err
	}

	// Authentication: providers, session store, and the login flow.
	svc := services.New(db.DB, cfg)
	sessions := auth.NewSessionService(redisClient,
		time.Duration(cfg.Auth.SessionDuration)*time.Second)

	providers := auth.NewRegistry()

	// What to say at the end of startup, when somebody is actually looking.
	var signIn []string

	// Google, when credentials are configured. Missing credentials are not
	// fatal: the provider is simply not offered.
	if cfg.OAuth.Google.ClientID != "" {
		google, err := auth.NewGoogleProvider(
			cfg.OAuth.Google.ClientID,
			cfg.OAuth.Google.ClientSecret,
			cfg.OAuth.Google.RedirectURL,
			"", // hosted domain: leave to the allow-list
		)
		if err != nil {
			return nil, fmt.Errorf("configuring google provider: %w", err)
		}
		providers.Add(google)
		log.Println("Google login enabled")
		signIn = append(signIn, "Google login enabled")
	}

	// Sign-in by emailed code, when this deployment has turned it on.
	//
	// Delivery is decided here rather than inside the store: a configured mail
	// server is used, and only in debug mode may a server fall back to writing
	// codes to its own log.
	var otpStore *auth.OTPStore
	if cfg.Auth.OTP.Enabled {
		mailer, describedAs, err := auth.NewMailer(auth.MailerSettings{
			BrevoAPIKey:   cfg.Auth.Brevo.APIKey,
			BrevoFrom:     cfg.Auth.Brevo.From,
			BrevoFromName: cfg.Auth.Brevo.FromName,
			SMTPHost:      cfg.Auth.SMTP.Host,
			SMTPPort:      cfg.Auth.SMTP.Port,
			SMTPUsername:  cfg.Auth.SMTP.Username,
			SMTPPassword:  cfg.Auth.SMTP.Password,
			SMTPFrom:      cfg.Auth.SMTP.From,
			SMTPFromName:  cfg.Auth.SMTP.FromName,
			ServerMode:    cfg.Server.Mode,
		})
		if err != nil {
			return nil, fmt.Errorf("code sign-in is enabled but %w", err)
		}

		if _, viaLog := mailer.(auth.LogMailer); viaLog {
			log.Printf("WARNING: code sign-in is writing codes to %s", describedAs)
			signIn = append(signIn,
				fmt.Sprintf("WARNING: sign-in codes are written to %s", describedAs))
		} else {
			log.Printf("code sign-in enabled, sending through %s", describedAs)
			signIn = append(signIn,
				fmt.Sprintf("sign-in codes sent through %s", describedAs))
		}

		otpStore = auth.NewOTPStore(redisClient, mailer)
		providers.Add(auth.NewOTPProvider(otpStore))
	}

	// Development stand-in, refused outside debug mode.
	if dev := auth.NewDevProvider(); dev.Guard(cfg.Server.Mode) == nil {
		providers.Add(dev)
		log.Println("WARNING: development login provider is enabled (server.mode=debug)")
		signIn = append(signIn,
			"WARNING: the development login is enabled - it accepts any email "+
				"with no password (server.mode=debug)")
	}

	if providers.Len() == 0 {
		log.Println("WARNING: no login providers are enabled - nobody can authenticate")
		signIn = append(signIn,
			"WARNING: nobody can sign in - no login method is configured")
	}

	// The allow-list lives in the database so the console can change it. Seed
	// it from configuration the first time only: that carries across the
	// domain the installer took from the first administrator's address, and
	// every domain an existing deployment already had, without the config
	// file overwriting the console's changes on every restart.
	if err := svc.Domains.SeedFromConfig(context.Background(), cfg.Auth.AllowedDomains); err != nil {
		return nil, fmt.Errorf("seeding the domain allow-list: %w", err)
	}

	authSvc := auth.NewService(providers, sessions, svc, redisClient, cfg.Auth.AllowedDomains)
	if otpStore != nil {
		authSvc.EnableOTP(otpStore)
	}

	// Emergency access. The links are minted by the CLI on this machine; all
	// the server does is redeem them.
	authSvc.EnableBreakGlass(auth.NewBreakGlass(redisClient, sessions, svc.Users, svc.Audit))

	// OpenVPN's control channel, which is the only way to act on a tunnel that
	// is already established. It gives two things: deactivating somebody ends
	// the session they are holding rather than only the next one, and a
	// completed login applies itself instead of asking the person to
	// reconnect by hand.
	//
	// Not fatal if absent. An older server, or one whose config predates the
	// management line, simply behaves as it did before.
	mgmt := vpn.NewManagement(cfg.OpenVPN.Management.Host, cfg.OpenVPN.Management.Port)
	svc.Users.EnableDisconnect(mgmt)
	authSvc.EnableReconnect(mgmt)

	// Close tunnels that were opened and never signed in on. Holding a
	// certificate opens a tunnel; only signing in earns access, and nothing
	// used to end the gap between the two.
	sweeper := vpn.NewIdleSweeper(mgmt, authSvc.SignedIn,
		time.Duration(cfg.Auth.CaptivePortalTimeout)*time.Second)
	if sweeper != nil {
		sweeper.OnReap = func(c vpn.ConnectedClient, idle time.Duration) {
			svc.Audit.Log(context.Background(), services.Entry{
				Action:       "vpn_idle_disconnected",
				ResourceType: "vpn_session",
				IPAddress:    c.VirtualAddress,
				Details: map[string]interface{}{
					"common_name":     c.CommonName,
					"idle_minutes":    int(idle.Minutes()),
					"never_signed_in": true,
				},
			})
		}
	}

	// Initialize router
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Believe X-Forwarded-For only from configured proxies. With none set,
	// ClientIP() is the actual peer address and cannot be spoofed.
	if err := router.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, fmt.Errorf("configuring trusted proxies: %w", err)
	}
	if len(cfg.Server.TrustedProxies) > 0 {
		log.Printf("trusting X-Forwarded-For from %v", cfg.Server.TrustedProxies)
	}
	router.LoadHTMLGlob("web/templates/*.html")

	app := &Dvarpala{
		signIn:   signIn,
		sweeper:  sweeper,
		config:   cfg,
		db:       db,
		redis:    redisClient,
		services: svc,
		auth:     authSvc,
		router:   router,
	}

	// Setup routes
	app.setupRoutes()

	return app, nil
}

func (d *Dvarpala) Router() *gin.Engine {
	return d.router
}

// SignInSummary describes how people may authenticate.
//
// Reported after the routes rather than only when it is decided: gin logs a
// line per route in between, so the one line an operator restarts the service
// to read had already scrolled past the last twenty.
func (d *Dvarpala) SignInSummary() []string { return d.signIn }

// Background starts the work that runs alongside the HTTP server, and returns
// when the context is cancelled. Safe to call when nothing is configured.
func (d *Dvarpala) Background(ctx context.Context) {
	d.sweeper.Run(ctx)
}

func (d *Dvarpala) setupRoutes() {
	// API routes
	apiGroup := d.router.Group("/api/v1")
	v1.SetupRoutes(apiGroup, d.services, d.redis, d.config)

	// Endpoints the OpenVPN hooks call. Localhost only in a real deployment.
	vpnapi.NewHandler(d.auth, d.services).Register(d.router.Group(""))

	// Web routes
	webGroup := d.router.Group("")
	web.SetupRoutes(webGroup, d.auth, d.services, d.config)

	// Anything else is answered by the portal. Clients inside the walled
	// garden have their web requests redirected here whatever they asked for,
	// so the paths that arrive are whatever they happened to be opening.
	d.router.NoRoute(web.NotFound(d.auth))
}
