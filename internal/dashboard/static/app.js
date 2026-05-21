// Forge Dashboard — Real-time WebSocket client
(function() {
    // Clock
    function updateClock() {
        const el = document.getElementById('clock');
        if (el) el.textContent = new Date().toLocaleTimeString();
    }
    setInterval(updateClock, 1000);
    updateClock();

    // WebSocket connection
    let ws = null;
    let reconnectDelay = 1000;

    function connectWS() {
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = proto + '//' + location.host + '/ws';

        ws = new WebSocket(url);

        ws.onopen = function() {
            const indicator = document.getElementById('ws-status');
            if (indicator) {
                indicator.className = 'ws-indicator connected';
                indicator.title = 'Connected';
            }
            reconnectDelay = 1000;
        };

        ws.onclose = function() {
            const indicator = document.getElementById('ws-status');
            if (indicator) {
                indicator.className = 'ws-indicator disconnected';
                indicator.title = 'Disconnected';
            }
            setTimeout(connectWS, reconnectDelay);
            reconnectDelay = Math.min(reconnectDelay * 2, 30000);
        };

        ws.onerror = function() {
            ws.close();
        };

        ws.onmessage = function(event) {
            try {
                const data = JSON.parse(event.data);
                handleEvent(data);
            } catch(e) {}
        };
    }

    function handleEvent(event) {
        // Update events list if present
        const eventsList = document.getElementById('events-list');
        if (eventsList) {
            const time = new Date(event.timestamp || Date.now()).toLocaleTimeString();
            const div = document.createElement('div');
            div.className = 'event-item event-' + event.type;
            div.innerHTML = '<span class="event-time">' + time + '</span>' +
                '<span class="event-type">' + event.type + '</span>' +
                '<span class="event-message">' + event.message + '</span>';
            eventsList.insertBefore(div, eventsList.firstChild);
            // Keep max 50 events
            while (eventsList.children.length > 50) {
                eventsList.removeChild(eventsList.lastChild);
            }
        }

        // Update metrics
        if (event.type === 'cost_update') {
            const el = document.getElementById('today-cost');
            if (el) el.textContent = event.message;
        }

        // Update agent count
        if (event.type === 'agent_start') {
            const el = document.getElementById('active-agents');
            if (el) el.textContent = parseInt(el.textContent || '0') + 1;
        }
        if (event.type === 'agent_stop') {
            const el = document.getElementById('active-agents');
            if (el) el.textContent = Math.max(0, parseInt(el.textContent || '0') - 1);
        }

        // Flash effect on new events
        document.querySelectorAll('.metric-card').forEach(function(card) {
            card.style.borderColor = '#f0883e';
            setTimeout(function() { card.style.borderColor = ''; }, 500);
        });
    }

    // Periodic metrics refresh
    function refreshMetrics() {
        fetch('/api/metrics')
            .then(function(r) { return r.json(); })
            .then(function(data) {
                updateIfPresent('active-agents', data.active_agents);
                updateIfPresent('running-sessions', data.running_sessions);
                updateIfPresent('total-sessions', data.total_sessions);
                updateIfPresent('uptime', data.uptime);
                if (data.today_cost !== undefined) {
                    const el = document.getElementById('today-cost');
                    if (el) el.textContent = '$' + data.today_cost.toFixed(2);
                }
                if (data.today_tokens !== undefined) {
                    const el = document.getElementById('today-tokens');
                    if (el) el.textContent = data.today_tokens.toLocaleString();
                }
            })
            .catch(function() {});
    }

    function updateIfPresent(id, value) {
        const el = document.getElementById(id);
        if (el && value !== undefined) el.textContent = value;
    }

    // Auto-refresh every 5 seconds
    setInterval(refreshMetrics, 5000);

    // Connect WebSocket
    connectWS();
})();
