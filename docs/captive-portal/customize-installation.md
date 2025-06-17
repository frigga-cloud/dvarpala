# Customizing Dvarpala Captive Portal During Installation

## 🎨 Pre-Installation Customization

### Method 1: Modify Installation Script

1. **Download the installation script**:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/provisioning/setup-server.sh -o setup-server.sh
   ```

2. **Edit the captive portal HTML** (around line 1040-1065):
   ```bash
   nano setup-server.sh
   ```

3. **Find this section and customize**:
   ```html
   html := `<!DOCTYPE html>
   <html>
   <head>
       <title>Dvarpala VPN - Authentication Required</title>
       <style>
           /* Customize CSS here */
           body { 
               font-family: Arial, sans-serif; 
               background: #f5f5f5; 
               margin: 0; 
               padding: 20px; 
           }
           .container { 
               max-width: 500px; 
               margin: 0 auto; 
               background: white; 
               padding: 30px; 
               border-radius: 10px; 
               box-shadow: 0 2px 10px rgba(0,0,0,0.1); 
           }
           /* Add your custom styles */
       </style>
   </head>
   <body>
       <div class="container">
           <h1>🔐 Your Custom VPN Title</h1>
           <p>Your custom message here.</p>
           <!-- Customize content -->
       </div>
   </body>
   </html>`
   ```

4. **Run the modified installation script**:
   ```bash
   sudo bash setup-server.sh
   ```

### Method 2: Create Custom Application

1. **Create your own main.go** with custom templates
2. **Replace the simple-main.go section** in the installation script
3. **Include external CSS/JS files** for advanced styling

## 🖼️ Customization Examples

### Corporate Branding
```css
.container {
    background: linear-gradient(135deg, #your-color1, #your-color2);
    color: white;
}

.logo {
    background-image: url('/static/images/your-logo.png');
    background-size: contain;
    background-repeat: no-repeat;
    height: 60px;
}
```

### Dark Theme
```css
body {
    background: #1a1a1a;
    color: #fff;
}

.container {
    background: #2d2d2d;
    border: 1px solid #444;
}

.auth-btn {
    background: #444;
    border: 1px solid #666;
}
```

### Mobile-First Design
```css
@media (max-width: 768px) {
    .container {
        margin: 10px;
        padding: 20px;
    }
    
    .auth-btn {
        font-size: 16px;
        padding: 15px;
    }
}
```

## 🎯 Key Elements to Customize

1. **Company Branding**:
   - Logo/Company name
   - Brand colors
   - Corporate messaging

2. **Layout & Design**:
   - Background colors/images
   - Button styles
   - Typography
   - Spacing and layout

3. **Content**:
   - Welcome message
   - Instructions
   - Footer information
   - Language localization

4. **Functionality**:
   - Loading animations
   - Error handling
   - Accessibility features
   - Analytics tracking

## 📂 File Structure After Customization

```
/opt/dvarpala/
├── templates/
│   ├── captive-portal.html      # Main login page
│   ├── auth-success.html        # Success page
│   └── auth-error.html          # Error page
├── web/
│   └── static/
│       ├── css/
│       │   └── captive-portal.css
│       ├── js/
│       │   └── captive-portal.js
│       └── images/
│           ├── logo.png
│           └── provider-icons/
└── bin/
    └── dvarpala-server          # Updated binary
```

## 🔄 Applying Updates

After customizing templates:

1. **Restart the Dvarpala service**:
   ```bash
   sudo systemctl restart dvarpala
   ```

2. **Clear browser cache** and test at:
   ```
   http://192.168.100.1:8080
   ```

3. **Check logs** for any errors:
   ```bash
   journalctl -u dvarpala -f
   ```