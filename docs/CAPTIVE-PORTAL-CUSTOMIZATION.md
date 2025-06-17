# Dvarpala Captive Portal Customization Guide

This guide explains how to customize the Dvarpala captive portal using the new modular HTML/JavaScript structure with Tailwind CSS.

## 🏗️ New Architecture Overview

The captive portal is now completely separated into standalone files that are easy to modify:

### 📁 File Structure
```
/opt/dvarpala/web/
├── templates/                 # HTML templates
│   ├── captive-portal.html   # Main authentication page
│   ├── auth-success.html     # Success page after authentication
│   └── auth-error.html       # Error page for failed authentication
├── static/
│   ├── js/
│   │   ├── captive-portal.js # Main JavaScript functionality
│   │   └── custom-portal.js  # Your custom JavaScript
│   ├── css/
│   │   ├── custom-portal.css # Your custom CSS
│   │   └── tailwind.css      # Tailwind CSS (CDN loaded)
│   └── images/               # Your custom images/logos
└── config/
    └── portal-config.json    # Configuration settings
```

### 🔄 How It Works

1. **Template Loading**: Go application loads HTML templates from `/opt/dvarpala/web/templates/`
2. **Static Files**: CSS, JS, and images served from `/opt/dvarpala/web/static/`
3. **Configuration**: JSON config file allows easy customization without code changes
4. **Fallback**: If templates aren't found, fallback to embedded HTML (for safety)

## 🚀 Quick Deployment

### Deploy Templates After Installation

```bash
# Deploy the new modular captive portal
sudo bash deploy-captive-portal.sh
```

This creates the complete template structure and restarts the Dvarpala service.

### Verify Deployment

```bash
# Check if templates are loaded
journalctl -u dvarpala | grep -i template

# Test the portal
curl -I http://192.168.100.1:8080
```

## 🎨 Customization Guide

### 1. Basic Configuration (No Code Changes)

Edit the configuration file to change basic settings:

```bash
sudo nano /opt/dvarpala/web/config/portal-config.json
```

**Example Configuration:**
```json
{
  "branding": {
    "company_name": "Acme Corporation",
    "logo_emoji": "🏢",
    "primary_color": "#1f2937",
    "secondary_color": "#3b82f6",
    "accent_color": "#10b981"
  },
  "portal": {
    "title": "Acme VPN - Secure Access",
    "subtitle": "Enterprise Network Gateway",
    "welcome_message": "Welcome to Acme's secure network. Please authenticate to continue.",
    "session_info": "Your session will remain active until you disconnect."
  },
  "support": {
    "email": "it-support@acme.com",
    "phone": "+1 (555) 123-ACME",
    "slack": "#it-support",
    "show_contact_info": true
  }
}
```

### 2. HTML Template Customization

#### Modify the Main Portal (`captive-portal.html`)

```bash
sudo nano /opt/dvarpala/web/templates/captive-portal.html
```

**Key sections to customize:**

**Header/Branding:**
```html
<div class="text-center mb-8">
    <div class="inline-flex items-center justify-center w-16 h-16 bg-white/20 rounded-full mb-4">
        <span class="text-3xl">🏢</span>  <!-- Change emoji -->
    </div>
    <h1 class="text-3xl font-bold text-white mb-2">Acme VPN</h1>  <!-- Change title -->
    <p class="text-white/80 text-sm font-medium tracking-wide">Enterprise Gateway</p>  <!-- Change subtitle -->
</div>
```

**Authentication Message:**
```html
<div class="text-center mb-6">
    <h2 class="text-2xl font-semibold text-gray-800 mb-2">Authentication Required</h2>
    <p class="text-gray-600 text-sm leading-relaxed">
        Welcome to Acme's secure network. Please authenticate to continue.  <!-- Customize message -->
    </p>
</div>
```

**OAuth Buttons:**
```html
<!-- Add/remove buttons, change styling -->
<button onclick="authenticateWithProvider('google')" 
        class="auth-btn w-full flex items-center justify-center gap-3 p-4 border border-gray-200 rounded-xl hover:bg-gray-50">
    <!-- Custom button content -->
</button>
```

### 3. CSS Customization with Tailwind

#### Using Tailwind Classes

The portal uses Tailwind CSS for styling. You can modify classes directly in the HTML:

```html
<!-- Original -->
<div class="bg-blue-600 text-white py-3 px-4 rounded-lg">

<!-- Customized -->
<div class="bg-green-600 text-white py-4 px-6 rounded-xl shadow-lg hover:bg-green-700 transition-all">
```

#### Custom CSS File

Add your own styles in `/opt/dvarpala/web/static/css/custom-portal.css`:

```css
/* Company Branding */
.company-brand {
    background: linear-gradient(135deg, #1f2937, #3b82f6);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
}

/* Custom Button Styles */
.auth-btn-acme {
    background: linear-gradient(135deg, #1f2937, #3b82f6);
    border: none;
    color: white;
    transition: all 0.3s ease;
}

.auth-btn-acme:hover {
    transform: translateY(-2px);
    box-shadow: 0 10px 30px rgba(31, 41, 55, 0.3);
}

/* Custom Animation */
.fade-in-custom {
    animation: fadeInCustom 1s ease-out;
}

@keyframes fadeInCustom {
    from { opacity: 0; transform: translateY(30px) scale(0.95); }
    to { opacity: 1; transform: translateY(0) scale(1); }
}
```

### 4. JavaScript Customization

#### Modify Behavior (`captive-portal.js`)

```bash
sudo nano /opt/dvarpala/web/static/js/captive-portal.js
```

**Custom Authentication Handler:**
```javascript
// Add to the CaptivePortal class
customAuthHandler(provider) {
    // Add custom logic before authentication
    console.log(`Starting ${provider} authentication for Acme Corp`);
    
    // Show custom loading message
    this.showStatusMessage(`Connecting to Acme ${provider} SSO...`, 'info');
    
    // Call original authentication
    this.authenticateWithProvider(provider);
}

// Custom success handler
handleAuthSuccess(data) {
    // Add custom success logic
    this.showCustomSuccessMessage(data);
    
    // Call original handler
    super.handleAuthSuccess(data);
}
```

#### Add Custom JavaScript

Create `/opt/dvarpala/web/static/js/custom-portal.js`:

```javascript
// Acme Corporation Custom Portal JavaScript

// Add company-specific functionality
document.addEventListener('DOMContentLoaded', function() {
    // Custom analytics
    trackPageView('captive-portal');
    
    // Custom welcome message based on time
    showTimeBasedWelcome();
    
    // Add company keyboard shortcuts
    setupAcmeKeyboardShortcuts();
});

function trackPageView(page) {
    // Add your analytics tracking
    console.log(`Acme Analytics: ${page} viewed`);
}

function showTimeBasedWelcome() {
    const hour = new Date().getHours();
    let greeting = 'Welcome';
    
    if (hour < 12) greeting = 'Good morning';
    else if (hour < 18) greeting = 'Good afternoon';
    else greeting = 'Good evening';
    
    // Update welcome message
    const welcomeElement = document.querySelector('.welcome-message');
    if (welcomeElement) {
        welcomeElement.textContent = `${greeting}! Please authenticate to access Acme's network.`;
    }
}

function setupAcmeKeyboardShortcuts() {
    document.addEventListener('keydown', function(e) {
        // Alt+H for help
        if (e.altKey && e.key === 'h') {
            showHelpModal();
        }
        
        // Alt+S for support
        if (e.altKey && e.key === 's') {
            window.open('mailto:it-support@acme.com');
        }
    });
}
```

### 5. Advanced Customization Examples

#### Corporate Theme

```html
<!-- Dark corporate theme with custom gradients -->
<body class="min-h-screen bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900">
    <div class="bg-gray-800 border border-gray-700 rounded-2xl shadow-2xl">
        <!-- Corporate styling -->
    </div>
</body>
```

#### Multi-language Support

```javascript
// Add to captive-portal.js
const translations = {
    en: {
        title: 'Authentication Required',
        welcome: 'Please authenticate to gain access',
        signin: 'Sign in with'
    },
    es: {
        title: 'Autenticación Requerida',
        welcome: 'Por favor autentíquese para obtener acceso',
        signin: 'Iniciar sesión con'
    }
};

function setLanguage(lang) {
    const t = translations[lang] || translations.en;
    document.querySelector('.auth-title').textContent = t.title;
    document.querySelector('.welcome-message').textContent = t.welcome;
}
```

#### Custom Provider Integration

```html
<!-- Add custom SSO provider -->
<button onclick="authenticateWithProvider('okta')" 
        class="auth-btn w-full bg-blue-800 text-white rounded-xl p-4">
    <div class="flex items-center gap-3">
        <img src="/static/images/okta-icon.svg" class="w-5 h-5">
        <span>Continue with Okta SSO</span>
    </div>
</button>
```

## 🔧 Testing and Deployment

### 1. Test Your Changes

```bash
# Restart the service to reload templates
sudo systemctl restart dvarpala

# Check logs for any errors
journalctl -u dvarpala -f

# Test the portal in a browser
# Access: http://192.168.100.1:8080 (from captive portal network)
```

### 2. Validate HTML/CSS

```bash
# Check HTML syntax
tidy -errors /opt/dvarpala/web/templates/captive-portal.html

# Validate CSS
csslint /opt/dvarpala/web/static/css/custom-portal.css
```

### 3. Performance Testing

```bash
# Test load time
curl -w "@curl-format.txt" -o /dev/null -s http://192.168.100.1:8080

# Check resource loading
curl -I http://192.168.100.1:8080/static/js/captive-portal.js
```

## 🛡️ Security Considerations

### 1. File Permissions

```bash
# Ensure proper permissions
sudo chown -R dvarpala:dvarpala /opt/dvarpala/web
sudo chmod -R 755 /opt/dvarpala/web
sudo chmod -R 644 /opt/dvarpala/web/**/*
```

### 2. Input Validation

- Never add user input directly to templates without validation
- Sanitize any dynamic content
- Use Content Security Policy (CSP) headers

### 3. HTTPS Considerations

For production with HTTPS:

```html
<!-- Update CDN links to HTTPS -->
<script src="https://cdn.tailwindcss.com"></script>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet">
```

## 🔄 Backup and Rollback

### Create Backup

```bash
# Backup current templates
sudo tar -czf dvarpala-portal-backup-$(date +%Y%m%d).tar.gz /opt/dvarpala/web/

# Store in safe location
sudo mv dvarpala-portal-backup-*.tar.gz /opt/dvarpala/backups/
```

### Rollback to Default

```bash
# Remove custom templates (will use embedded fallback)
sudo rm -rf /opt/dvarpala/web/templates/*

# Restart service
sudo systemctl restart dvarpala
```

## 📞 Support and Troubleshooting

### Common Issues

1. **Templates not loading**: Check file permissions and paths
2. **JavaScript errors**: Check browser console for errors
3. **CSS not applying**: Verify Tailwind CDN is accessible
4. **Service won't start**: Check logs with `journalctl -u dvarpala`

### Getting Help

- Check application logs: `journalctl -u dvarpala -f`
- Verify template loading: Look for template-related log messages
- Test fallback: Remove templates to ensure embedded HTML works

---

## 🎯 Quick Customization Checklist

- [ ] Deploy templates with `deploy-captive-portal.sh`
- [ ] Update `portal-config.json` with your branding
- [ ] Modify `captive-portal.html` with your company info
- [ ] Add custom CSS to `custom-portal.css`
- [ ] Test authentication flow
- [ ] Add custom JavaScript if needed
- [ ] Set proper file permissions
- [ ] Restart Dvarpala service
- [ ] Test from captive portal network

**The captive portal is now fully modular and easy to customize! 🎉**