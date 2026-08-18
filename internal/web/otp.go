package web

import (
	"html/template"
	"net/http"
	"net/url"

	"dvarpala/internal/auth"

	"github.com/gin-gonic/gin"
)

// OTPHandler serves the two steps of signing in with an emailed code.
//
// It lives outside /auth/ so it cannot collide with the /auth/:provider
// routes, exactly as the development login form does.
type OTPHandler struct {
	auth *auth.Service
}

// NewOTPHandler creates the handler.
func NewOTPHandler(a *auth.Service) *OTPHandler { return &OTPHandler{auth: a} }

// Register attaches the code sign-in routes.
func (h *OTPHandler) Register(r *gin.RouterGroup) {
	r.GET("/otp/login", h.Form)
	r.POST("/otp/request", h.Request)
	r.POST("/otp/verify", h.Verify)
}

// Form asks for an email address.
func (h *OTPHandler) Form(c *gin.Context) {
	h.render(c, otpView{Step: "email", State: c.Query("state")})
}

// Request sends a code, then asks for it.
//
// The answer is the same whether or not the address can sign in - see
// Service.RequestCode. Only a rate limit or a mail failure changes what the
// person is told, because those are the two things they can act on.
func (h *OTPHandler) Request(c *gin.Context) {
	email := c.PostForm("email")
	state := c.PostForm("state")

	view := otpView{Step: "code", State: state, Email: email}

	if err := h.auth.RequestCode(c.Request.Context(), email, clientIP(c)); err != nil {
		view.Step = "email"
		view.Error = err.Error()
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
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = otpLoginPage.Execute(c.Writer, v)
}

// otpLoginPage is inline rather than a file in web/templates, because those
// are loaded into one shared namespace by LoadHTMLGlob and this page belongs
// to the login flow rather than to the portal's own set.
var otpLoginPage = template.Must(template.New("otp").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign in to Dvarpala</title>
<style>
  body { font-family: -apple-system, "Segoe UI", Roboto, sans-serif;
         background: #f4f5f8; color: #1b2029; display: flex;
         min-height: 100vh; align-items: center; justify-content: center; margin: 0; }
  .card { background: #fff; border-radius: 12px; padding: 32px; width: 100%;
          max-width: 380px; box-shadow: 0 1px 3px rgba(0,0,0,.08); }
  h1 { font-size: 20px; margin: 0 0 6px; }
  p  { color: #5b6572; font-size: 14px; line-height: 1.5; margin: 0 0 20px; }
  label { display: block; font-size: 13px; font-weight: 600; margin-bottom: 6px; }
  input { width: 100%; box-sizing: border-box; padding: 11px 12px; font-size: 15px;
          border: 1px solid #d4d9e0; border-radius: 7px; }
  input.code { letter-spacing: 8px; text-align: center; font-size: 22px; }
  button { width: 100%; margin-top: 16px; padding: 11px; font-size: 15px;
           font-weight: 600; color: #fff; background: #2f6fdb; border: 0;
           border-radius: 7px; cursor: pointer; }
  .err { background: #fdeceb; color: #97271d; border-radius: 7px;
         padding: 10px 12px; font-size: 13px; margin-bottom: 16px; }
</style>
</head>
<body>
  <div class="card">
  {{if .Error}}<div class="err">{{.Error}}</div>{{end}}
  {{if eq .Step "code"}}
    <h1>Check your email</h1>
    <p>If {{.Email}} can sign in, a six-digit code is on its way. It expires in five minutes.</p>
    <form method="post" action="/otp/verify">
      <input type="hidden" name="state" value="{{.State}}">
      <input type="hidden" name="email" value="{{.Email}}">
      <label for="code">Sign-in code</label>
      <input class="code" id="code" name="code" inputmode="numeric" autocomplete="one-time-code"
             maxlength="6" required autofocus>
      <button type="submit">Sign in</button>
    </form>
  {{else}}
    <h1>Sign in to Dvarpala</h1>
    <p>Enter your work email address and we will send you a code.</p>
    <form method="post" action="/otp/request">
      <input type="hidden" name="state" value="{{.State}}">
      <label for="email">Work email</label>
      <input id="email" name="email" type="email" autocomplete="email" required autofocus>
      <button type="submit">Send me a code</button>
    </form>
  {{end}}
  </div>
</body>
</html>`))
