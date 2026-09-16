        // Tab Logic
        function switchTab(tabName) {
            document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
            
            event.currentTarget.classList.add('active');
            document.getElementById('tab-' + tabName).classList.add('active');
        }

        const btnConnect = document.getElementById('btnConnect');
        const btnDisconnect = document.getElementById('btnDisconnect');
        const btnAuth = document.getElementById('btnAuth');
        const statusCard = document.getElementById('statusCard');
        const statusText = document.getElementById('statusText');

        setInterval(() => {
            updateStatus();
            fetchTunnels(false); // background refresh for tunnels state
        }, 2000);

        async function updateStatus() {
            try {
                const res = await fetch('/api/status');
                const data = await res.json();
                if (data.connected) {
                    setUIState('connected');
                } else if (statusCard.classList.contains('connected')) {
                    setUIState('disconnected');
                }
            } catch (e) {
                console.error("Error fetching status", e);
            }
        }

        function setUIState(state, authUrl = null) {
            statusCard.className = `glass-card status-card ${state}`;
            btnConnect.classList.remove('loading');
            btnDisconnect.classList.remove('loading');

            btnConnect.style.display = 'none';
            btnDisconnect.style.display = 'none';
            btnAuth.style.display = 'none';

            if (state === 'connected') {
                statusText.innerText = 'VPN Connected';
                btnDisconnect.style.display = 'flex';
            } else if (state === 'connecting') {
                statusText.innerText = 'Starting...';
                btnConnect.style.display = 'flex';
                btnConnect.classList.add('loading');
            } else if (state === 'auth_required') {
                statusText.innerText = 'Authentication Required';
                btnAuth.href = authUrl;
                btnAuth.style.display = 'flex';
                statusCard.className = `glass-card status-card connecting`;
            } else {
                statusText.innerText = 'Disconnected';
                btnConnect.style.display = 'flex';
            }
        }

        async function startVPN() {
            if (btnConnect.classList.contains('loading')) return;
            setUIState('connecting');

            try {
                const res = await fetch('/api/start', { method: 'POST' });
                const data = await res.json();

                if (data.url) {
                    setUIState('auth_required', data.url);
                } else if (data.status === 'connected') {
                    setUIState('connected');
                } else {
                    alert('Error: ' + (data.error || 'Could not start'));
                    setUIState('disconnected');
                }
            } catch (e) {
                alert('Backend connection failed.');
                setUIState('disconnected');
            }
        }

        async function stopVPN() {
            if (btnDisconnect.classList.contains('loading')) return;
            btnDisconnect.classList.add('loading');
            try {
                await fetch('/api/stop', { method: 'POST' });
            } catch (e) { }
        }

        // ====================
        // PROXIES LOGIC
        // ====================
        async function fetchProxies() {
            try {
                const res = await fetch('/api/proxies');
                const rules = await res.json();
                const list = document.getElementById('proxyList');
                list.innerHTML = '';

                if (rules && rules.length > 0) {
                    rules.forEach(rule => {
                        let dotClass = rule.status === 'active' ? 'active' : '';
                        list.innerHTML += `
                            <div class="proxy-card">
                                <div class="proxy-info">
                                    <div class="proxy-port"><span class="status-dot ${dotClass}"></span> Port ${rule.port}</div>
                                    <div class="proxy-target">${rule.target}</div>
                                </div>
                                <div class="proxy-actions">
                                    <button class="icon-btn" onclick="editProxy(${rule.port}, '${rule.target}')" title="Edit">
                                        <svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"></path></svg>
                                    </button>
                                    <button class="icon-btn delete" onclick="deleteProxy(${rule.port})" title="Delete">
                                        <svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                    </button>
                                </div>
                            </div>
                        `;
                    });
                } else {
                    list.innerHTML = '<div style="text-align: center; color: var(--text-muted); padding: 20px;">No forwarded ports active</div>';
                }
            } catch (e) { }
        }

        async function addProxy() {
            const portInput = document.getElementById('proxyPort');
            const targetInput = document.getElementById('proxyTarget');
            const port = parseInt(portInput.value);
            let target = targetInput.value.trim();

            if (!port || !target) {
                alert("Please provide both port and target URL.");
                return;
            }
            if (!target.startsWith('http://') && !target.startsWith('https://')) {
                target = 'http://' + target;
            }

            try {
                const res = await fetch('/api/proxies', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ port, target })
                });

                if (res.ok) {
                    portInput.value = '';
                    targetInput.value = '';
                    document.getElementById('btnSubmitText').innerText = 'Add';
                    fetchProxies();
                } else {
                    const err = await res.text();
                    alert("Error saving proxy: " + err);
                }
            } catch (e) {
                alert("Error connecting to server.");
            }
        }

        function editProxy(port, target) {
            document.getElementById('proxyPort').value = port;
            document.getElementById('proxyTarget').value = target;
            document.getElementById('btnSubmitText').innerText = 'Save';
            document.getElementById('proxyTarget').focus();
        }

        async function deleteProxy(port) {
            try {
                const res = await fetch('/api/proxies?port=' + port, { method: 'DELETE' });
                if (res.ok) fetchProxies();
            } catch (e) { }
        }


        // ====================
        // TUNNELS LOGIC
        // ====================
        let tunnelsData = [];

        async function fetchTunnels(showLoading = true) {
            if(showLoading) {
                document.getElementById('tunnelList').innerHTML = '<div style="text-align: center; color: var(--text-muted); padding: 20px;">Loading...</div>';
            }
            
            try {
                const res = await fetch('/api/tunnels');
                tunnelsData = await res.json();
                renderTunnels();
            } catch (e) { }
        }

        function renderTunnels() {
            const list = document.getElementById('tunnelList');
            list.innerHTML = '';

            if (tunnelsData && tunnelsData.length > 0) {
                tunnelsData.forEach(t => {
                    let dotClass = '';
                    if(t.status === 'connected') dotClass = 'active';
                    else if(t.status === 'connecting') dotClass = 'connecting';
                    else if(t.status && t.status.startsWith('error')) dotClass = 'error';

                    let actBtn = `<button class="icon-btn start" onclick="toggleTunnelStatus('${t.id}', 'start')" title="Start"><svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"></path><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg></button>`;
                    
                    if (t.status === 'connected' || t.status === 'connecting') {
                        actBtn = `<button class="icon-btn delete" onclick="toggleTunnelStatus('${t.id}', 'stop')" title="Stop"><svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10h6v4H9z"></path></svg></button>`;
                    }

                    let errTitle = (t.status && t.status.startsWith('error')) ? `title="${t.status}"` : "";

                    list.innerHTML += `
                        <div class="proxy-card">
                            <div class="proxy-info">
                                <div class="tunnel-name" ${errTitle}>
                                    <span class="status-dot ${dotClass}"></span> 
                                    ${t.name}
                                    <span class="tunnel-type">${t.type.toUpperCase()}</span>
                                </div>
                                <div class="proxy-target">Local: ${t.local_port} → Remote: ${t.remote_host}:${t.remote_port}</div>
                            </div>
                            <div class="proxy-actions">
                                ${actBtn}
                                <button class="icon-btn" onclick="editTunnel('${t.id}')" title="Edit">
                                    <svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"></path></svg>
                                </button>
                                <button class="icon-btn delete" onclick="deleteTunnel('${t.id}')" title="Delete">
                                    <svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                </button>
                            </div>
                        </div>
                    `;
                });
            } else {
                list.innerHTML = '<div style="text-align: center; color: var(--text-muted); padding: 20px;">No tunnels configured</div>';
            }
        }

        async function toggleTunnelStatus(id, action) {
            try {
                await fetch(`/api/tunnels/${id}/${action}`, { method: 'POST' });
                fetchTunnels(false);
            } catch (e) { }
        }

        async function deleteTunnel(id) {
            if(!confirm("Are you sure you want to delete this tunnel?")) return;
            try {
                await fetch(`/api/tunnels/${id}`, { method: 'DELETE' });
                fetchTunnels(false);
            } catch (e) { }
        }

        // Modal Logic
        function toggleTunnelFields() {
            const type = document.getElementById('tunType').value;
            document.getElementById('fields-aws-ssm').classList.remove('show');
            document.getElementById('fields-ssh').classList.remove('show');
            
            if (type === 'aws-ssm') document.getElementById('fields-aws-ssm').classList.add('show');
            if (type === 'ssh') document.getElementById('fields-ssh').classList.add('show');
        }

        function openTunnelModal() {
            document.getElementById('tunnelForm').reset();
            document.getElementById('tunnelId').value = "";
            document.getElementById('modalTitle').innerText = "Add Tunnel";
            toggleTunnelFields();
            document.getElementById('tunnelModal').classList.add('show');
        }

        function closeTunnelModal() {
            document.getElementById('tunnelModal').classList.remove('show');
        }

        function editTunnel(id) {
            const t = tunnelsData.find(x => x.id === id);
            if (!t) return;
            
            document.getElementById('modalTitle').innerText = "Edit Tunnel";
            document.getElementById('tunnelId').value = t.id;
            document.getElementById('tunName').value = t.name;
            document.getElementById('tunType').value = t.type;
            document.getElementById('tunRemotePort').value = t.remote_port;
            document.getElementById('tunLocalPort').value = t.local_port;
            document.getElementById('tunRemoteHost').value = t.remote_host;
            
            if (t.type === 'aws-ssm') {
                document.getElementById('tunAwsProfile').value = t.aws_profile || "";
                let tagsStr = "";
                if(t.target_tags) {
                    tagsStr = Object.entries(t.target_tags).map(([k,v]) => `${k}=${v}`).join(",");
                }
                document.getElementById('tunTargetTags').value = tagsStr;
            } else if (t.type === 'ssh') {
                document.getElementById('tunJumpHost').value = t.jump_host || "";
                document.getElementById('tunJumpUser').value = t.jump_user || "";
                document.getElementById('tunSSHKey').value = t.ssh_key_file || "";
            }

            toggleTunnelFields();
            document.getElementById('tunnelModal').classList.add('show');
        }

        async function saveTunnel(e) {
            e.preventDefault();
            
            const id = document.getElementById('tunnelId').value || crypto.randomUUID();
            const type = document.getElementById('tunType').value;
            
            const payload = {
                id: id,
                name: document.getElementById('tunName').value,
                type: type,
                remote_port: parseInt(document.getElementById('tunRemotePort').value),
                local_port: parseInt(document.getElementById('tunLocalPort').value),
                remote_host: document.getElementById('tunRemoteHost').value,
            };

            if (type === 'aws-ssm') {
                payload.aws_profile = document.getElementById('tunAwsProfile').value;
                const tagStr = document.getElementById('tunTargetTags').value;
                if (tagStr) {
                    payload.target_tags = {};
                    tagStr.split(',').forEach(kv => {
                        const parts = kv.split('=');
                        if(parts.length===2) payload.target_tags[parts[0].trim()] = parts[1].trim();
                    });
                }
            } else if (type === 'ssh') {
                payload.jump_host = document.getElementById('tunJumpHost').value;
                payload.jump_user = document.getElementById('tunJumpUser').value;
                payload.ssh_key_file = document.getElementById('tunSSHKey').value;
            }

            const isEdit = !!document.getElementById('tunnelId').value;
            const url = isEdit ? `/api/tunnels/${id}` : `/api/tunnels`;
            const method = isEdit ? `PUT` : `POST`;

            try {
                const res = await fetch(url, {
                    method: method,
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });

                if (res.ok) {
                    closeTunnelModal();
                    fetchTunnels(true);
                } else {
                    const err = await res.text();
                    alert("Error saving tunnel: " + err);
                }
            } catch (e) {
                alert("Error connecting to server.");
            }
        }

        // Init
        updateStatus();
        fetchProxies();
        fetchTunnels(true);
