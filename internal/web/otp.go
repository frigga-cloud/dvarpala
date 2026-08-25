package web

import (
	"errors"
	"net/http"
	"net/url"

	"dvarpala/internal/auth"

	"github.com/gin-gonic/gin"
)

// OTPHandler serves signing in with an emailed code.
//
// It lives outside /auth/ because those routes are all one provider each,
// and these are the pages of a flow rather than a provider of their own -
// the same reason the development login form sits outside.
type OTPHandler struct {
	auth *auth.Service
}

// NewOTPHandler creates the handler.
func NewOTPHandler(a *auth.Service) *OTPHandler { return &OTPHandler{auth: a} }

// Register attaches the code sign-in routes.
func (h *OTPHandler) Register(r *gin.RouterGroup) {
	r.POST("/otp/start", h.Start)
	r.GET("/otp/login", h.Form)
	r.POST("/otp/request", h.Request)
	r.POST("/otp/verify", h.Verify)
}

// Start is the portal's own entry point: an address arrives, a login attempt
// is created for it, and a code goes out - all from one button.
//
// The other providers begin at /auth/:provider, which redirects somewhere
// else to collect an identity. There is nowhere else to send anyone here, so
// the attempt is created in place instead.
func (h *OTPHandler) Start(c *gin.Context) {
	email := c.PostForm("email")

	state, err := h.auth.NewLoginAttempt(c.Request.Context(), "otp", clientIP(c))
	if err != nil {
		// Back to the portal with the reason, rather than onward to a code
		// form that could never work.
		c.HTML(http.StatusOK, "captive-portal.html", portalView(h.auth, err.Error()))
		return
	}

	h.send(c, email, state)
}

// Form asks for an email address.
//
// Reached when a login begins at /auth/otp rather than from the portal - the
// generic provider route, which every provider has.
func (h *OTPHandler) Form(c *gin.Context) {
	h.render(c, otpView{Step: "email", State: c.Query("state")})
}

// Request sends a code for an attempt that already exists. This is the
// "send it again" path, and the form's own submit.
func (h *OTPHandler) Request(c *gin.Context) {
	h.send(c, c.PostForm("email"), c.PostForm("state"))
}

// send asks for a code and shows whichever step comes next.
//
// The answer is the same whether or not the address can sign in - see
// Service.RequestCode. Only a rate limit or a mail failure changes what the
// person is told, because those are the two things they can act on.
func (h *OTPHandler) send(c *gin.Context, email, state string) {
	view := otpView{Step: "code", State: state, Email: email}

	if err := h.auth.RequestCode(c.Request.Context(), email, clientIP(c)); err != nil {
		view.Error = err.Error()

		// Being told to wait means a code was just sent, so the person has a
		// working one in front of them - keep them on the step where they can
		// type it. Only send them back to the start for errors that make a
		// code impossible, such as the mail server refusing it.
		//
		// This is not hypothetical tidying. A page that submits twice, which
		// captive-portal browsers do, sent a code on the first attempt and hit
		// the cooldown on the second - and the second reply replaced the code
		// form with "wait before asking for another" and an empty email box.
		// The code was in their inbox and there was nowhere to type it.
		if !errors.Is(err, auth.ErrTooSoon) {
			view.Step = "email"
		}
	}
	h.render(c, view)
}

// Verify checks the code and hands back to the ordinary login flow.
//
// What travels through the redirect is the state token, not the address: only
// an attempt that presented the right code has a verification recorded against
// it, so knowing somebody's email is not enough to sign in as them.
func (h *OTPHandler) Verify(c *gin.Context) {
	email := c.PostForm("email")
	code := c.PostForm("code")
	state := c.PostForm("state")

	if err := h.auth.VerifyCode(c.Request.Context(), email, code, state); err != nil {
		h.render(c, otpView{Step: "code", State: state, Email: email, Error: err.Error()})
		return
	}

	c.Redirect(http.StatusFound, "/auth/otp/callback?code="+url.QueryEscape(state)+
		"&state="+url.QueryEscape(state))
}

// otpView is everything the sign-in page needs.
type otpView struct {
	// Step is "email" or "code".
	Step  string
	State string
	Email string
	Error string
}

func (h *OTPHandler) render(c *gin.Context, v otpView) {
	c.HTML(http.StatusOK, "otp-login.html", gin.H{
		"step": v.Step, "state": v.State, "email": v.Email, "error": v.Error,
	})
}
