#!/bin/bash
# Script to customize Dvarpala captive portal after installation

DVARPALA_HOME="/opt/dvarpala"
TEMPLATES_DIR="$DVARPALA_HOME/templates"
WEB_DIR="$DVARPALA_HOME/web"

echo "🎨 Customizing Dvarpala Captive Portal..."

# Create directories for custom templates and assets
sudo mkdir -p "$TEMPLATES_DIR"
sudo mkdir -p "$WEB_DIR/static/css"
sudo mkdir -p "$WEB_DIR/static/js"
sudo mkdir -p "$WEB_DIR/static/images"

# Create custom captive portal template
sudo tee "$TEMPLATES_DIR/captive-portal.html" > /dev/null << 'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>🔐 Dvarpala VPN Authentication</title>
    <link rel="stylesheet" href="/static/css/captive-portal.css">
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600&display=swap" rel="stylesheet">
</head>
<body>
    <div class="background">
        <div class="container">
            <div class="header">
                <div class="logo">
                    <h1>🛡️ Dvarpala</h1>
                    <p class="subtitle">Zero Trust VPN Gateway</p>
                </div>
            </div>
            
            <div class="auth-card">
                <h2>Authentication Required</h2>
                <p class="description">
                    Welcome to the secure network. Please authenticate with your organization's identity provider to gain access.
                </p>
                
                <div class="auth-providers">
                    <a href="/auth/google" class="auth-btn google">
                        <img src="/static/images/google-icon.svg" alt="Google" class="provider-icon">
                        <span>Continue with Google</span>
                    </a>
                    
                    <a href="/auth/microsoft" class="auth-btn microsoft">
                        <img src="/static/images/microsoft-icon.svg" alt="Microsoft" class="provider-icon">
                        <span>Continue with Microsoft</span>
                    </a>
                    
                    <a href="/auth/github" class="auth-btn github">
                        <img src="/static/images/github-icon.svg" alt="GitHub" class="provider-icon">
                        <span>Continue with GitHub</span>
                    </a>
                    
                    <a href="/auth/gitlab" class="auth-btn gitlab">
                        <img src="/static/images/gitlab-icon.svg" alt="GitLab" class="provider-icon">
                        <span>Continue with GitLab</span>
                    </a>
                </div>
                
                <div class="info">
                    <p>
                        <span class="info-icon">ℹ️</span>
                        After successful authentication, you will receive full network access until you disconnect.
                    </p>
                </div>
            </div>
            
            <div class="footer">
                <p>&copy; 2024 Your Organization. Secured by Dvarpala VPN.</p>
            </div>
        </div>
    </div>
    
    <script src="/static/js/captive-portal.js"></script>
</body>
</html>
EOF

# Create custom CSS
sudo tee "$WEB_DIR/static/css/captive-portal.css" > /dev/null << 'EOF'
/* Dvarpala Custom Captive Portal Styles */

* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

body {
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    line-height: 1.6;
    color: #333;
    overflow-x: hidden;
}

.background {
    min-height: 100vh;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
}

.background::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: url('data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><defs><pattern id="grid" width="10" height="10" patternUnits="userSpaceOnUse"><path d="M 10 0 L 0 0 0 10" fill="none" stroke="rgba(255,255,255,0.1)" stroke-width="0.5"/></pattern></defs><rect width="100" height="100" fill="url(%23grid)"/></svg>');
    opacity: 0.3;
}

.container {
    background: rgba(255, 255, 255, 0.95);
    backdrop-filter: blur(10px);
    border-radius: 20px;
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
    padding: 40px;
    max-width: 480px;
    width: 90%;
    position: relative;
    z-index: 1;
}

.header {
    text-align: center;
    margin-bottom: 30px;
}

.logo h1 {
    font-size: 2.5em;
    font-weight: 600;
    background: linear-gradient(135deg, #667eea, #764ba2);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    margin-bottom: 5px;
}

.subtitle {
    color: #666;
    font-size: 0.95em;
    font-weight: 300;
    letter-spacing: 0.5px;
}

.auth-card h2 {
    color: #333;
    font-size: 1.5em;
    font-weight: 500;
    text-align: center;
    margin-bottom: 15px;
}

.description {
    color: #666;
    text-align: center;
    margin-bottom: 30px;
    line-height: 1.5;
}

.auth-providers {
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-bottom: 25px;
}

.auth-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 14px 20px;
    border-radius: 12px;
    text-decoration: none;
    font-weight: 500;
    transition: all 0.3s ease;
    border: 1px solid transparent;
    position: relative;
    overflow: hidden;
}

.auth-btn::before {
    content: '';
    position: absolute;
    top: 0;
    left: -100%;
    width: 100%;
    height: 100%;
    background: linear-gradient(90deg, transparent, rgba(255,255,255,0.2), transparent);
    transition: left 0.5s;
}

.auth-btn:hover::before {
    left: 100%;
}

.auth-btn.google {
    background: #fff;
    color: #333;
    border-color: #ddd;
}

.auth-btn.google:hover {
    background: #f8f9fa;
    box-shadow: 0 4px 12px rgba(0,0,0,0.1);
    transform: translateY(-2px);
}

.auth-btn.microsoft {
    background: #0078d4;
    color: white;
}

.auth-btn.microsoft:hover {
    background: #106ebe;
    box-shadow: 0 4px 12px rgba(16,110,190,0.3);
    transform: translateY(-2px);
}

.auth-btn.github {
    background: #333;
    color: white;
}

.auth-btn.github:hover {
    background: #24292e;
    box-shadow: 0 4px 12px rgba(36,41,46,0.3);
    transform: translateY(-2px);
}

.auth-btn.gitlab {
    background: #fc6d26;
    color: white;
}

.auth-btn.gitlab:hover {
    background: #e24329;
    box-shadow: 0 4px 12px rgba(252,109,38,0.3);
    transform: translateY(-2px);
}

.provider-icon {
    width: 20px;
    height: 20px;
}

.info {
    background: #f8f9fa;
    border-left: 4px solid #667eea;
    border-radius: 6px;
    padding: 15px;
    margin-top: 20px;
}

.info p {
    color: #555;
    font-size: 0.9em;
    display: flex;
    align-items: flex-start;
    gap: 8px;
}

.info-icon {
    font-size: 1.1em;
    margin-top: 1px;
}

.footer {
    text-align: center;
    margin-top: 25px;
    padding-top: 20px;
    border-top: 1px solid #eee;
}

.footer p {
    color: #999;
    font-size: 0.85em;
}

/* Responsive Design */
@media (max-width: 600px) {
    .container {
        padding: 30px 25px;
        margin: 20px;
    }
    
    .logo h1 {
        font-size: 2em;
    }
    
    .auth-btn {
        padding: 12px 16px;
    }
}

/* Loading animation */
.auth-btn.loading {
    pointer-events: none;
    opacity: 0.7;
}

.auth-btn.loading::after {
    content: '';
    width: 16px;
    height: 16px;
    border: 2px solid transparent;
    border-top-color: currentColor;
    border-radius: 50%;
    animation: spin 1s linear infinite;
    margin-left: 8px;
}

@keyframes spin {
    to { transform: rotate(360deg); }
}
EOF

# Create JavaScript for enhanced functionality
sudo tee "$WEB_DIR/static/js/captive-portal.js" > /dev/null << 'EOF'
// Dvarpala Captive Portal JavaScript

document.addEventListener('DOMContentLoaded', function() {
    // Add loading state to auth buttons
    const authButtons = document.querySelectorAll('.auth-btn');
    
    authButtons.forEach(button => {
        button.addEventListener('click', function(e) {
            // Add loading state
            this.classList.add('loading');
            
            // Remove loading state after 10 seconds (in case of redirect failure)
            setTimeout(() => {
                this.classList.remove('loading');
            }, 10000);
        });
    });
    
    // Add subtle animations
    const container = document.querySelector('.container');
    container.style.opacity = '0';
    container.style.transform = 'translateY(20px)';
    
    setTimeout(() => {
        container.style.transition = 'all 0.6s ease';
        container.style.opacity = '1';
        container.style.transform = 'translateY(0)';
    }, 100);
    
    // Add keyboard navigation
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Enter' && e.target.classList.contains('auth-btn')) {
            e.target.click();
        }
    });
});
EOF

# Create authentication success template
sudo tee "$TEMPLATES_DIR/auth-success.html" > /dev/null << 'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>✅ Authentication Successful - Dvarpala VPN</title>
    <link rel="stylesheet" href="/static/css/captive-portal.css">
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600&display=swap" rel="stylesheet">
</head>
<body>
    <div class="background">
        <div class="container">
            <div class="header">
                <div class="logo">
                    <h1>🛡️ Dvarpala</h1>
                    <p class="subtitle">Zero Trust VPN Gateway</p>
                </div>
            </div>
            
            <div class="auth-card">
                <div style="text-align: center; margin-bottom: 20px;">
                    <div style="font-size: 4em; margin-bottom: 10px;">✅</div>
                    <h2 style="color: #22c55e;">Authentication Successful!</h2>
                </div>
                
                <div class="success-info">
                    <p><strong>Welcome, {{.username}}!</strong></p>
                    <p>You have been successfully authenticated via {{.provider}}.</p>
                    <p>You now have full network access and can browse the internet normally.</p>
                </div>
                
                <div class="info" style="background: #f0fdf4; border-left-color: #22c55e;">
                    <p>
                        <span class="info-icon">🔒</span>
                        Your session will remain active until you disconnect from the VPN. 
                        You will need to re-authenticate when you reconnect.
                    </p>
                </div>
                
                <div style="text-align: center; margin-top: 25px;">
                    <a href="#" onclick="window.close()" style="color: #667eea; text-decoration: none; font-weight: 500;">
                        Close this window
                    </a>
                </div>
            </div>
        </div>
    </div>
</body>
</html>
EOF

# Set proper permissions
sudo chown -R dvarpala:dvarpala "$TEMPLATES_DIR"
sudo chown -R dvarpala:dvarpala "$WEB_DIR"
sudo chmod -R 644 "$TEMPLATES_DIR"/*
sudo chmod -R 644 "$WEB_DIR"/static/*

echo "✅ Custom captive portal templates created!"
echo ""
echo "📁 Template locations:"
echo "   - Captive Portal: $TEMPLATES_DIR/captive-portal.html"
echo "   - Success Page: $TEMPLATES_DIR/auth-success.html"
echo "   - CSS: $WEB_DIR/static/css/captive-portal.css"
echo "   - JavaScript: $WEB_DIR/static/js/captive-portal.js"
echo ""
echo "🔄 To apply changes, you'll need to update the Dvarpala application"
echo "   to use these templates instead of the embedded HTML."
echo ""
echo "💡 You can modify these files to customize:"
echo "   - Colors and branding"
echo "   - Logo and company information"
echo "   - Layout and styling"
echo "   - Additional JavaScript functionality"