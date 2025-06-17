#!/bin/bash
# Deploy Dvarpala Captive Portal Templates
# This script deploys the standalone HTML/JavaScript captive portal

set -e

DVARPALA_HOME="/opt/dvarpala"
WEB_DIR="$DVARPALA_HOME/web"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🚀 Deploying Dvarpala Captive Portal Templates..."

# Check if running as root or with dvarpala user permissions
if [[ $EUID -eq 0 ]]; then
    SUDO_CMD="sudo"
    USER_CMD="sudo -u dvarpala"
elif [[ $(whoami) == "dvarpala" ]]; then
    SUDO_CMD=""
    USER_CMD=""
else
    echo "❌ This script must be run as root or dvarpala user"
    exit 1
fi

# Create web directories
echo "📁 Creating web directories..."
$SUDO_CMD mkdir -p "$WEB_DIR/templates"
$SUDO_CMD mkdir -p "$WEB_DIR/static/js"
$SUDO_CMD mkdir -p "$WEB_DIR/static/css"
$SUDO_CMD mkdir -p "$WEB_DIR/static/images"
$SUDO_CMD mkdir -p "$WEB_DIR/config"

# Copy templates
echo "📄 Deploying HTML templates..."
if [ -f "$SCRIPT_DIR/web/templates/captive-portal.html" ]; then
    $SUDO_CMD cp "$SCRIPT_DIR/web/templates/captive-portal.html" "$WEB_DIR/templates/"
    echo "   ✅ captive-portal.html"
else
    echo "   ⚠️  captive-portal.html not found, creating minimal template"
    $SUDO_CMD tee "$WEB_DIR/templates/captive-portal.html" > /dev/null << 'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>🔐 Dvarpala VPN - Authentication Required</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="min-h-screen bg-gradient-to-br from-blue-600 to-purple-700 flex items-center justify-center p-4">
    <div class="bg-white rounded-2xl p-8 shadow-2xl max-w-md w-full">
        <div class="text-center mb-8">
            <div class="text-4xl mb-4">🛡️</div>
            <h1 class="text-2xl font-bold text-gray-800 mb-2">Dvarpala VPN</h1>
            <p class="text-gray-600">Authentication Required</p>
        </div>
        
        <div class="space-y-3">
            <a href="/auth/google" class="block w-full bg-white border border-gray-300 text-gray-700 py-3 px-4 rounded-lg text-center hover:bg-gray-50 transition-colors">
                Sign in with Google
            </a>
            <a href="/auth/microsoft" class="block w-full bg-blue-600 text-white py-3 px-4 rounded-lg text-center hover:bg-blue-700 transition-colors">
                Sign in with Microsoft
            </a>
            <a href="/auth/github" class="block w-full bg-gray-900 text-white py-3 px-4 rounded-lg text-center hover:bg-gray-800 transition-colors">
                Sign in with GitHub
            </a>
            <a href="/auth/gitlab" class="block w-full bg-orange-600 text-white py-3 px-4 rounded-lg text-center hover:bg-orange-700 transition-colors">
                Sign in with GitLab
            </a>
        </div>
        
        <div class="mt-6 p-4 bg-blue-50 rounded-lg">
            <p class="text-sm text-blue-800">
                <span class="font-medium">ℹ️ Session Info:</span>
                After authentication, you'll receive full network access until disconnect.
            </p>
        </div>
    </div>
</body>
</html>
EOF
fi

if [ -f "$SCRIPT_DIR/web/templates/auth-success.html" ]; then
    $SUDO_CMD cp "$SCRIPT_DIR/web/templates/auth-success.html" "$WEB_DIR/templates/"
    echo "   ✅ auth-success.html"
fi

if [ -f "$SCRIPT_DIR/web/templates/auth-error.html" ]; then
    $SUDO_CMD cp "$SCRIPT_DIR/web/templates/auth-error.html" "$WEB_DIR/templates/"
    echo "   ✅ auth-error.html"
fi

# Copy JavaScript
echo "📜 Deploying JavaScript files..."
if [ -f "$SCRIPT_DIR/web/static/js/captive-portal.js" ]; then
    $SUDO_CMD cp "$SCRIPT_DIR/web/static/js/captive-portal.js" "$WEB_DIR/static/js/"
    echo "   ✅ captive-portal.js"
else
    echo "   ⚠️  captive-portal.js not found, creating minimal JavaScript"
    $SUDO_CMD tee "$WEB_DIR/static/js/captive-portal.js" > /dev/null << 'EOF'
// Dvarpala Captive Portal JavaScript - Minimal Version
console.log('🔐 Dvarpala Captive Portal loaded');

// Add loading states to auth buttons
document.addEventListener('DOMContentLoaded', function() {
    const authButtons = document.querySelectorAll('a[href^="/auth/"]');
    
    authButtons.forEach(button => {
        button.addEventListener('click', function(e) {
            this.style.opacity = '0.7';
            this.innerHTML = '⏳ Authenticating...';
        });
    });
});
EOF
fi

# Copy configuration
echo "⚙️  Deploying configuration..."
if [ -f "$SCRIPT_DIR/web/config/portal-config.json" ]; then
    $SUDO_CMD cp "$SCRIPT_DIR/web/config/portal-config.json" "$WEB_DIR/config/"
    echo "   ✅ portal-config.json"
fi

# Create a basic CSS file if it doesn't exist
if [ ! -f "$WEB_DIR/static/css/custom-portal.css" ]; then
    echo "🎨 Creating custom CSS file..."
    $SUDO_CMD tee "$WEB_DIR/static/css/custom-portal.css" > /dev/null << 'EOF'
/* Dvarpala Custom Portal Styles */

/* You can add custom CSS here to override Tailwind styles */

.dvarpala-brand {
    background: linear-gradient(135deg, #6366f1, #8b5cf6);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
}

.auth-btn-custom {
    transition: all 0.3s ease;
}

.auth-btn-custom:hover {
    transform: translateY(-2px);
    box-shadow: 0 10px 25px rgba(0, 0, 0, 0.1);
}

/* Loading spinner */
.loading-spinner {
    animation: spin 1s linear infinite;
}

@keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
}

/* Custom animations */
.fade-in {
    animation: fadeIn 0.6s ease-in-out;
}

@keyframes fadeIn {
    from { opacity: 0; transform: translateY(20px); }
    to { opacity: 1; transform: translateY(0); }
}
EOF
    echo "   ✅ custom-portal.css"
fi

# Set proper permissions
echo "🔒 Setting permissions..."
$SUDO_CMD chown -R dvarpala:dvarpala "$WEB_DIR"
$SUDO_CMD chmod -R 755 "$WEB_DIR"
$SUDO_CMD chmod -R 644 "$WEB_DIR"/**/*
$SUDO_CMD find "$WEB_DIR" -type d -exec chmod 755 {} \;

# Restart Dvarpala service if it's running
echo "🔄 Restarting Dvarpala service..."
if systemctl is-active --quiet dvarpala; then
    $SUDO_CMD systemctl restart dvarpala
    echo "   ✅ Service restarted"
else
    echo "   ⚠️  Dvarpala service not running"
fi

echo ""
echo "✅ Deployment completed successfully!"
echo ""
echo "📂 Template files deployed to:"
echo "   📁 $WEB_DIR/templates/"
echo "   📁 $WEB_DIR/static/"
echo "   📁 $WEB_DIR/config/"
echo ""
echo "🌐 Access the captive portal at:"
echo "   🔗 http://192.168.100.1:8080/ (captive portal network)"
echo "   🔗 http://YOUR_SERVER_IP:8080/ (if accessible directly)"
echo ""
echo "🛠️  To customize the portal:"
echo "   1. Edit files in $WEB_DIR/templates/"
echo "   2. Modify $WEB_DIR/config/portal-config.json"
echo "   3. Add custom CSS to $WEB_DIR/static/css/"
echo "   4. Restart: sudo systemctl restart dvarpala"
echo ""
echo "📚 Files you can customize:"
echo "   📄 captive-portal.html - Main login page"
echo "   📄 auth-success.html - Success page"
echo "   📄 auth-error.html - Error page"
echo "   📜 captive-portal.js - JavaScript functionality"
echo "   🎨 custom-portal.css - Custom styles"
echo "   ⚙️  portal-config.json - Configuration settings"
echo ""
echo "🎉 Happy customizing!"