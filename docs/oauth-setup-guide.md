# OAuth Provider Setup Guide

This guide helps you set up OAuth applications for each supported provider before running the Dvarpala installation.

## 🔧 Provider Setup Instructions

### 1. Google/Gmail OAuth Setup

1. **Go to Google Cloud Console**
   - Visit: https://console.cloud.google.com/

2. **Create or Select Project**
   - Create a new project or select an existing one
   - Enable the Google+ API or Google Identity API

3. **Create OAuth Credentials**
   - Navigate to: APIs & Services > Credentials
   - Click "Create Credentials" > "OAuth 2.0 Client IDs"
   - Set Application type: "Web application"
   - Set Name: "Dvarpala VPN"

4. **Configure Redirect URI**
   - Add Authorized redirect URI: `http://YOUR_SERVER_IP:8080/auth/google/callback`
   - Replace `YOUR_SERVER_IP` with your actual server IP

5. **Save Credentials**
   - Copy the Client ID and Client Secret
   - You'll need these during installation

---

### 2. Microsoft OAuth Setup

1. **Go to Azure Portal**
   - Visit: https://portal.azure.com/

2. **Navigate to App Registrations**
   - Go to: Azure Active Directory > App registrations
   - Click "New registration"

3. **Register Application**
   - Name: "Dvarpala VPN"
   - Supported account types: "Accounts in any organizational directory and personal Microsoft accounts"
   - Redirect URI: `http://YOUR_SERVER_IP:8080/auth/microsoft/callback`

4. **Create Client Secret**
   - Go to: Certificates & secrets
   - Click "New client secret"
   - Set Description: "Dvarpala VPN Secret"
   - Set Expires: Choose appropriate duration

5. **Save Credentials**
   - Copy the Application (client) ID from Overview
   - Copy the Client Secret value (not the ID)

---

### 3. GitHub OAuth Setup

1. **Go to GitHub Developer Settings**
   - Visit: https://github.com/settings/applications/new

2. **Create OAuth App**
   - Application name: "Dvarpala VPN"
   - Homepage URL: `http://YOUR_SERVER_IP:8080`
   - Authorization callback URL: `http://YOUR_SERVER_IP:8080/auth/github/callback`

3. **Generate Client Secret**
   - After creating the app, generate a new client secret

4. **Save Credentials**
   - Copy the Client ID
   - Copy the Client Secret

---

### 4. GitLab OAuth Setup

1. **Go to GitLab Applications**
   - For GitLab.com: https://gitlab.com/-/profile/applications
   - For self-hosted: `https://YOUR_GITLAB_URL/-/profile/applications`

2. **Create Application**
   - Name: "Dvarpala VPN"
   - Redirect URI: `http://YOUR_SERVER_IP:8080/auth/gitlab/callback`
   - Scopes: Select "read_user" (minimum required)

3. **Save Application**
   - Copy the Application ID
   - Copy the Secret

4. **Note GitLab URL**
   - For GitLab.com: use `https://gitlab.com`
   - For self-hosted: use your GitLab instance URL

---

## 📋 Installation Checklist

Before running the Dvarpala installation, prepare:

### Required Information
- [ ] **Server IP Address** - Your VPN server's public IP
- [ ] **OAuth Provider Choice** - Select up to 2 providers
- [ ] **OAuth Credentials** - Client ID and Secret for each chosen provider

### Example Preparation Sheet

```
Server IP: _______________

Provider 1: ☐ Google ☐ Microsoft ☐ GitHub ☐ GitLab
Client ID: ________________________________
Client Secret: ____________________________

Provider 2: ☐ Google ☐ Microsoft ☐ GitHub ☐ GitLab
Client ID: ________________________________
Client Secret: ____________________________
GitLab URL (if applicable): ________________
```

## 🔐 Security Best Practices

### OAuth Application Security
- **Use specific redirect URIs** - Don't use wildcards
- **Rotate secrets regularly** - Update client secrets periodically
- **Monitor application usage** - Review OAuth app logs
- **Restrict scopes** - Only request necessary permissions

### Redirect URI Configuration
- **Development**: `http://localhost:8080/auth/PROVIDER/callback`
- **Production**: `https://your-domain.com/auth/PROVIDER/callback`
- **IP-based**: `http://YOUR_SERVER_IP:8080/auth/PROVIDER/callback`

### Provider-Specific Notes

#### Google
- Requires verification for production use with many users
- Consider Google Workspace integration for enterprise

#### Microsoft
- Supports both personal and organizational accounts
- Consider Azure AD integration for enterprise

#### GitHub
- Limited to public email addresses by default
- Consider GitHub Enterprise for private repositories

#### GitLab
- Supports both GitLab.com and self-hosted instances
- Requires "read_user" scope minimum

## 🛠️ Testing OAuth Setup

After installation, test each provider:

1. **Connect to VPN** using temporary credentials
2. **Access web interface** at `http://YOUR_SERVER_IP:8080`
3. **Try OAuth login** for each configured provider
4. **Verify user information** is correctly retrieved

## 🆘 Troubleshooting

### Common Issues

#### Invalid Redirect URI
- **Problem**: OAuth provider rejects redirect
- **Solution**: Verify redirect URI matches exactly
- **Check**: Protocol (http/https), IP/domain, port, path

#### Invalid Client Credentials
- **Problem**: Authentication fails with provider
- **Solution**: Verify Client ID and Secret are correct
- **Check**: No extra spaces, special characters, or truncation

#### Scope Issues
- **Problem**: Unable to retrieve user information
- **Solution**: Verify required scopes are granted
- **Check**: Provider-specific scope requirements

#### Network Issues
- **Problem**: Cannot reach OAuth provider
- **Solution**: Check firewall and DNS settings
- **Check**: Outbound HTTPS access from server

---

**Next Step**: Once you have your OAuth credentials ready, proceed with the [Dvarpala Installation](../INSTALLATION.md).