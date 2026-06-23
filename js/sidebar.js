// Unified sidebar navigation
const sidebarHTML = `
<div class="sidebar-section">
    <h3 class="sidebar-title">Getting Started</h3>
    <ul class="sidebar-nav">
        <li><a href="/rue/docs/getting-started.html">Introduction</a></li>
        <li><a href="/rue/docs/installation.html">Installation</a></li>
        <li><a href="/rue/docs/quickstart.html">Quick Start</a></li>
    </ul>
</div>
<div class="sidebar-section">
    <h3 class="sidebar-title">Core Concepts</h3>
    <ul class="sidebar-nav">
        <li><a href="/rue/docs/routing.html">Routing</a></li>
        <li><a href="/rue/docs/context.html">Context</a></li>
        <li><a href="/rue/docs/middleware.html">Middleware</a></li>
        <li><a href="/rue/docs/binding.html">Data Binding</a></li>
        <li><a href="/rue/docs/validation.html">Validation</a></li>
        <li><a href="/rue/docs/templates.html">Templates</a></li>
        <li><a href="/rue/docs/logging.html">Logging</a></li>
    </ul>
</div>
<div class="sidebar-section">
    <h3 class="sidebar-title">Features</h3>
    <ul class="sidebar-nav">
        <li><a href="/rue/docs/websocket.html">WebSocket</a></li>
        <li><a href="/rue/docs/sse.html">SSE</a></li>
        <li><a href="/rue/docs/graphql.html">GraphQL</a></li>
        <li><a href="/rue/docs/grpc.html">gRPC</a></li>
        <li><a href="/rue/docs/quic.html">QUIC/HTTP3</a></li>
        <li><a href="/rue/docs/webrtc.html">WebRTC Signaling</a></li>
    </ul>
</div>
<div class="sidebar-section">
    <h3 class="sidebar-title">Advanced</h3>
    <ul class="sidebar-nav">
        <li><a href="/rue/docs/error-handling.html">Error Handling</a></li>
        <li><a href="/rue/docs/environment.html">Environment</a></li>
        <li><a href="/rue/docs/compression.html">Compression</a></li>
        <li><a href="/rue/docs/testing.html">Testing</a></li>
        <li><a href="/rue/docs/api-reference.html">API Reference</a></li>
    </ul>
</div>
`;

document.addEventListener('DOMContentLoaded', function() {
    const sidebar = document.getElementById('sidebar');
    if (sidebar) {
        sidebar.innerHTML = sidebarHTML;
        
        // Highlight current page
        const currentPath = window.location.pathname;
        const links = sidebar.querySelectorAll('a');
        links.forEach(link => {
            if (currentPath.endsWith(link.getAttribute('href').split('/').pop())) {
                link.classList.add('active');
            }
        });
    }
});
