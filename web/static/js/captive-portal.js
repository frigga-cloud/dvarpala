/**
 * Dvarpala Captive Portal JavaScript
 * Handles OAuth authentication flow and UI interactions
 */

class CaptivePortal {
    constructor() {
        this.init();
        this.setupEventListeners();
        this.checkAuthStatus();
    }

    init() {
        // Configuration
        this.config = {
            authEndpoints: {
                google: '/auth/google',
                microsoft: '/auth/microsoft',
                github: '/auth/github',
                gitlab: '/auth/gitlab'
            },
            statusCheckInterval: 5000, // 5 seconds
            maxRetries: 3,
            retryDelay: 2000 // 2 seconds
        };

        // State management
        this.state = {
            isAuthenticating: false,
            currentProvider: null,
            retryCount: 0
        };

        // UI elements
        this.elements = {
            authButtons: document.querySelectorAll('.auth-btn'),
            statusIndicator: document.getElementById('status-indicator'),
            authProviders: document.getElementById('auth-providers')
        };

        console.log('🔐 Dvarpala Captive Portal initialized');
    }

    setupEventListeners() {
        // Keyboard navigation
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && e.target.classList.contains('auth-btn')) {
                e.target.click();
            }
        });

        // Escape key to cancel authentication
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && this.state.isAuthenticating) {
                this.cancelAuthentication();
            }
        });

        // Visibility change detection (user switches tabs)
        document.addEventListener('visibilitychange', () => {
            if (!document.hidden && this.state.isAuthenticating) {
                this.checkAuthStatus();
            }
        });
    }

    async authenticateWithProvider(provider) {
        if (this.state.isAuthenticating) {
            console.warn('Authentication already in progress');
            return;
        }

        try {
            this.state.isAuthenticating = true;
            this.state.currentProvider = provider;
            this.state.retryCount = 0;

            console.log(`🚀 Starting authentication with ${provider}`);
            
            this.setLoadingState(provider, true);
            this.showStatusMessage(`Connecting to ${provider}...`, 'info');

            // Validate provider
            if (!this.config.authEndpoints[provider]) {
                throw new Error(`Unknown provider: ${provider}`);
            }

            // Small delay for better UX
            await this.delay(500);

            // Redirect to OAuth provider
            window.location.href = this.config.authEndpoints[provider];

        } catch (error) {
            console.error('Authentication error:', error);
            this.handleAuthError(error);
        }
    }

    setLoadingState(provider, isLoading) {
        const button = document.querySelector(`[data-provider="${provider}"]`);
        if (!button) return;

        if (isLoading) {
            button.classList.add('loading');
            button.disabled = true;
            // Disable all other buttons
            this.elements.authButtons.forEach(btn => {
                if (btn !== button) {
                    btn.disabled = true;
                    btn.classList.add('opacity-50', 'cursor-not-allowed');
                }
            });
        } else {
            button.classList.remove('loading');
            button.disabled = false;
            // Re-enable all buttons
            this.elements.authButtons.forEach(btn => {
                btn.disabled = false;
                btn.classList.remove('opacity-50', 'cursor-not-allowed');
            });
        }
    }

    showStatusMessage(message, type = 'info') {
        const indicator = this.elements.statusIndicator;
        const dot = indicator.querySelector('.w-3');
        const text = indicator.querySelector('span');

        // Set colors based on type
        const colors = {
            info: 'bg-blue-500 text-blue-100',
            success: 'bg-green-500 text-green-100',
            error: 'bg-red-500 text-red-100',
            warning: 'bg-yellow-500 text-yellow-100'
        };

        const dotColors = {
            info: 'bg-blue-300',
            success: 'bg-green-300',
            error: 'bg-red-300',
            warning: 'bg-yellow-300'
        };

        // Clear previous classes
        indicator.className = 'mt-4 p-4 rounded-xl text-center transition-all';
        dot.className = 'w-3 h-3 rounded-full animate-pulse';

        // Add new classes
        indicator.classList.add(colors[type]);
        dot.classList.add(dotColors[type]);
        text.textContent = message;

        // Show indicator
        indicator.classList.remove('hidden');

        // Auto-hide success/error messages
        if (type === 'success' || type === 'error') {
            setTimeout(() => {
                indicator.classList.add('hidden');
            }, 5000);
        }
    }

    hideStatusMessage() {
        this.elements.statusIndicator.classList.add('hidden');
    }

    async checkAuthStatus() {
        try {
            const response = await fetch('/api/internal/auth-status', {
                method: 'GET',
                credentials: 'same-origin'
            });

            if (response.ok) {
                const data = await response.json();
                
                if (data.authenticated) {
                    this.handleAuthSuccess(data);
                } else if (this.state.isAuthenticating) {
                    // Still waiting for authentication
                    setTimeout(() => this.checkAuthStatus(), this.config.statusCheckInterval);
                }
            }
        } catch (error) {
            console.warn('Auth status check failed:', error);
            if (this.state.isAuthenticating && this.state.retryCount < this.config.maxRetries) {
                this.state.retryCount++;
                setTimeout(() => this.checkAuthStatus(), this.config.retryDelay);
            }
        }
    }

    handleAuthSuccess(data) {
        console.log('✅ Authentication successful:', data);
        
        this.state.isAuthenticating = false;
        this.setLoadingState(this.state.currentProvider, false);
        
        this.showStatusMessage('Authentication successful! Granting network access...', 'success');
        
        // Show success animation
        this.showSuccessAnimation();
        
        // Redirect to success page or close window
        setTimeout(() => {
            window.location.href = '/auth/success';
        }, 2000);
    }

    handleAuthError(error) {
        console.error('❌ Authentication failed:', error);
        
        this.state.isAuthenticating = false;
        this.setLoadingState(this.state.currentProvider, false);
        
        const errorMessage = error.message || 'Authentication failed. Please try again.';
        this.showStatusMessage(errorMessage, 'error');
        
        // Reset state
        this.state.currentProvider = null;
    }

    cancelAuthentication() {
        console.log('🚫 Authentication cancelled by user');
        
        this.state.isAuthenticating = false;
        this.setLoadingState(this.state.currentProvider, false);
        this.hideStatusMessage();
        
        this.state.currentProvider = null;
    }

    showSuccessAnimation() {
        // Create success overlay
        const overlay = document.createElement('div');
        overlay.className = 'fixed inset-0 bg-green-500/20 flex items-center justify-center z-50 animate-fade-in';
        
        const successIcon = document.createElement('div');
        successIcon.className = 'bg-white rounded-full p-8 shadow-xl animate-pulse-slow';
        successIcon.innerHTML = '<span class="text-6xl">✅</span>';
        
        overlay.appendChild(successIcon);
        document.body.appendChild(overlay);
        
        // Remove after animation
        setTimeout(() => {
            overlay.remove();
        }, 2000);
    }

    // Utility methods
    delay(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }

    getProviderDisplayName(provider) {
        const names = {
            google: 'Google',
            microsoft: 'Microsoft',
            github: 'GitHub',
            gitlab: 'GitLab'
        };
        return names[provider] || provider;
    }

    // Public API methods
    retry() {
        if (this.state.currentProvider) {
            this.authenticateWithProvider(this.state.currentProvider);
        }
    }

    reset() {
        this.state.isAuthenticating = false;
        this.state.currentProvider = null;
        this.state.retryCount = 0;
        
        this.elements.authButtons.forEach(btn => {
            btn.disabled = false;
            btn.classList.remove('loading', 'opacity-50', 'cursor-not-allowed');
        });
        
        this.hideStatusMessage();
    }
}

// Global function for HTML onclick handlers
function authenticateWithProvider(provider) {
    if (window.captivePortal) {
        window.captivePortal.authenticateWithProvider(provider);
    }
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.captivePortal = new CaptivePortal();
    
    // Add any custom initialization here
    console.log('🔐 Dvarpala Captive Portal ready');
});

// Handle page unload
window.addEventListener('beforeunload', () => {
    if (window.captivePortal && window.captivePortal.state.isAuthenticating) {
        // Optionally cancel ongoing authentication
        window.captivePortal.cancelAuthentication();
    }
});

// Export for module systems (if needed)
if (typeof module !== 'undefined' && module.exports) {
    module.exports = CaptivePortal;
}