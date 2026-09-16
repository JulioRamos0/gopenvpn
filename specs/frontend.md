# Frontend Specification (GopenVPN)

## 1. Overview
The GopenVPN frontend is built as a lightweight, single-page application (SPA) designed with a "Material Design" aesthetic. The primary goal of the UI is to feel dynamic, premium, and fully integrated without relying on heavy external frameworks (like React or Vue) or large CSS libraries (like Tailwind).

It relies exclusively on **Vanilla HTML, CSS, and JavaScript**, packaged and served natively by the Go backend via the `//go:embed` directive.

## 2. File Structure

Recent structural refactors separated the monolith into three distinct files located in the `web/` directory:

- `index.html`: The semantic structure of the page, including the main app container, dynamic status cards, tabs, and the tunnel configuration modal.
- `style.css`: Contains all styling logic, CSS variables (design tokens), animations, and responsive breakpoints.
- `app.js`: Encapsulates all JavaScript logic, including state management, API polling, DOM manipulation, and dynamic HTML rendering.

## 3. Design System & Aesthetics (CSS)

The design embraces dark mode and a flat, Material-inspired approach.

### Design Tokens (CSS Variables)
- **Backgrounds**: Uses solid, flat dark backgrounds (`--bg-dark: #0b0f19` for the body, `--bg-surface: #1e2532` for cards and modals).
- **Material Effects**: Removes transparent blur effects in favor of solid surface colors and defined drop shadows (`--shadow-sm`, `--shadow-md`, `--shadow-lg`) to simulate elevation and depth.
- **Accents**: Cyan/Blue tones (`--accent-primary: #00f2fe`) for primary actions.
- **Semantic Colors**: Green for connected/active states (`--success`), yellow for connecting (`--warning`), and red for errors/disconnect (`--danger`).
- **Typography**: Uses the 'Outfit' font family imported via Google Fonts for a modern look.

### Animations
Micro-animations are used extensively to provide real-time feedback:
- `pulse`: Used on the status dots to indicate a "connecting" or transitioning state.
- `spin`: Used for loading indicators inside buttons.
- `fadeIn`: Used for tab content switching.

## 4. State Management and Logic (JS)

The frontend operates statelessly regarding persistence, deriving its "truth" from the Go backend via periodic polling.

### Core Mechanisms
1. **Status Polling**:
   - A `setInterval` loop runs every 2 seconds, invoking `updateStatus()` and `fetchTunnels()`.
   - `updateStatus()` hits `/api/status` to determine the global VPN state (`connected`, `disconnected`, `connecting`, or `auth_required`).
2. **UI State Transitions**:
   - `setUIState(state)` dynamically modifies classes on the `#statusCard`, manages button visibility, and updates text labels.
   - For `auth_required`, it dynamically reveals the SSO link.
3. **Tab Switching**:
   - Handled via `switchTab(tabName)`, applying `.active` classes to buttons and content blocks to show/hide the "Proxies" and "Tunnels" sections.

### Resource Managers (Proxies & Tunnels)
- **Proxies**: Handled by `fetchProxies()`. Dynamically maps JSON rules into HTML templates. If a proxy is active in the backend, its status dot is painted green.
- **Tunnels**: Handled by `fetchTunnels()`. Displays the real-time status (Connected/Stopped/Error) mapped from the backend `tunnel.GetStatus()`.
- **Tunnel Modal**: A dynamic modal form (`openTunnelModal`, `closeTunnelModal`). Depending on the selected `<select id="tunType">`, it conditionally reveals SSM parameters or SSH parameters via `toggleTunnelFields()`.

## 5. API Integration Map

The JavaScript fetches data asynchronously from the following Go endpoints:

| Feature | Method | Endpoint | Description |
|---|---|---|---|
| **VPN** | `POST` | `/api/start` | Initiates the VPN connection and returns SSO URL if required. |
| **VPN** | `POST` | `/api/stop` | Terminates the VPN connection and all bound processes. |
| **VPN** | `GET` | `/api/status` | Returns `{ "connected": bool }` based on `tun0` interface existence. |
| **Proxies** | `GET` | `/api/proxies` | Retrieves configured reverse proxies and their active status. |
| **Proxies** | `POST` | `/api/proxies` | Adds a new port-forwarding proxy rule. |
| **Proxies** | `DELETE` | `/api/proxies?port=` | Removes an existing proxy rule. |
| **Tunnels** | `GET` | `/api/tunnels` | Retrieves configured tunnels and their in-memory status. |
| **Tunnels** | `POST` | `/api/tunnels` | Creates a new tunnel definition. |
| **Tunnels** | `PUT` | `/api/tunnels/:id` | Updates an existing tunnel definition. |
| **Tunnels** | `DELETE` | `/api/tunnels/:id` | Deletes a tunnel definition. |
| **Tunnels** | `POST` | `/api/tunnels/:id/start`| Activates the tunnel (SSM or SSH). |
| **Tunnels** | `POST` | `/api/tunnels/:id/stop` | Terminates the active tunnel process. |
