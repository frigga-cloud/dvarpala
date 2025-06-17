# Dvarpala VPN

Dvarpala (द्वारपाल) - Sanskrit for "gatekeeper" - is an enterprise-grade VPN solution with Zero Trust Network Access (ZTNA) and granular access control.

## Features

- OpenVPN-based secure connection with captive portal
- Multi-provider OAuth authentication (Google, Microsoft, GitHub)
- Group-based access control with hierarchical permissions
- Dynamic route assignment based on user groups
- RESTful API for external tool integration
- Comprehensive admin dashboard
- Audit logging and session management

## Quick Start

1. **Setup Development Environment**
   ```bash
   make setup-dev
   ```

2. **Build the Application**
   ```bash
   make build
   ```

3. **Run Development Server**
   ```bash
   make run
   ```

4. **Run Tests**
   ```bash
   make test
   ```

## Project Structure

- `cmd/` - Main application entry points
- `internal/` - Private application code
- `pkg/` - Public packages
- `api/` - API definitions (OpenAPI, gRPC)
- `web/` - Web templates and static assets
- `configs/` - Configuration files
- `deployments/` - Docker, Kubernetes, Terraform configs
- `scripts/` - Build and deployment scripts
- `test/` - Test files and fixtures

## Documentation

See the [docs/](docs/) directory for detailed documentation.

## License

MIT License
