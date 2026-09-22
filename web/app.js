/* ===================================================================
   GOmegle — Client Application
   Omegle-style 1:1 Random Video & Text Chat
   Built with Go, WebSockets, and WebRTC
   =================================================================== */

(function () {
    'use strict';

    // --- State & Mode ---
    const State = {
        IDLE: 'idle',
        SEARCHING: 'searching',
        CHATTING: 'chatting',
        DISCONNECTED: 'disconnected',
    };

    const Mode = {
        VIDEO: 'video',
        TEXT: 'text',
    };

    // --- Configuration ---
    const rtcConfig = {
        iceServers: [
            { urls: 'stun:stun.l.google.com:19302' },
            { urls: 'stun:stun1.l.google.com:19302' },
            { urls: 'stun:stun2.l.google.com:19302' },
            { urls: 'stun:stun3.l.google.com:19302' },
            { urls: 'stun:stun4.l.google.com:19302' },
        ],
        iceCandidatePoolSize: 10,
    };

    // --- App Variables ---
    let state = State.IDLE;
    let mode = Mode.VIDEO;
    let ws = null;
    let interests = [];

    // WebRTC references
    let pc = null;
    let localStream = null;
    let isInitiator = false;
    let remoteDescriptionSet = false;
    let pendingRemoteCandidates = [];
    let isVideoEnabled = true;
    let isAudioEnabled = true;
    let currentFacingMode = 'user'; // 'user' or 'environment'

    // Typing & UI timers
    let typingTimeout = null;
    let isTyping = false;
    let statsInterval = null;
    let disconnectConfirmTimer = null;
    let isConfirmingDisconnect = false;

    // --- DOM Elements ---
    const $ = (id) => document.getElementById(id);

    const dom = {
        // Header
        modeBadge: $('mode-badge'),
        modeBadgeText: $('mode-badge-text'),
        onlineCount: $('online-count'),

        // Landing
        landingPanel: $('landing-panel'),
        startVideoBtn: $('start-video-btn'),
        startTextBtn: $('start-text-btn'),
        startBtn: $('start-btn'),
        interestInput: $('interest-input'),
        interestTags: $('interest-tags'),

        // Panels & Layout
        chatPanel: $('chat-panel'),
        chatLayout: $('chat-layout'),
        videoArea: $('video-area'),
        chatArea: $('chat-area'),

        // Video feeds
        remoteVideo: $('remote-video'),
        localVideo: $('local-video'),
        remotePlaceholder: $('remote-placeholder'),
        remotePlaceholderText: $('remote-placeholder-text'),
        localPlaceholder: $('local-placeholder'),
        remoteConnStatus: $('remote-conn-status'),
        remoteMicOff: $('remote-mic-off'),
        remoteCamOff: $('remote-cam-off'),

        // Local media controls
        toggleMicBtn: $('toggle-mic-btn'),
        toggleCamBtn: $('toggle-cam-btn'),
        flipCamBtn: $('flip-cam-btn'),

        // Chat & messaging
        statusBar: $('status-bar'),
        statusText: $('status-text'),
        commonInterests: $('common-interests'),
        commonInterestsList: $('common-interests-list'),
        chatLog: $('chat-log'),
        typingIndicator: $('typing-indicator'),
        messageInput: $('message-input'),
        sendBtn: $('send-btn'),

        // Chat action controls
        disconnectBtn: $('disconnect-btn'),
        disconnectBtnLabel: $('disconnect-btn-label'),
        newBtn: $('new-btn'),
        leaveHomeBtn: $('leave-home-btn'),

        // Media alert banner
        mediaAlert: $('media-alert'),
        mediaAlertTitle: $('media-alert-title'),
        mediaAlertMsg: $('media-alert-msg'),
        mediaRetryBtn: $('media-retry-btn'),
        mediaFallbackTextBtn: $('media-fallback-text-btn'),
        mediaDismissBtn: $('media-dismiss-btn'),
    };

    // ===================================================================
    //  Interest Tags Management
    // ===================================================================

    function addInterest(tag) {
        tag = tag.trim().toLowerCase();
        if (!tag || tag.length > 30 || interests.length >= 10) return;
        if (interests.includes(tag)) return;

        interests.push(tag);
        renderInterestTags();
    }

    function removeInterest(tag) {
        interests = interests.filter((t) => t !== tag);
        renderInterestTags();
    }

    function renderInterestTags() {
        dom.interestTags.innerHTML = '';
        interests.forEach((tag) => {
            const el = document.createElement('span');
            el.className = 'interest-tag';
            el.innerHTML =
                escapeHtml(tag) +
                '<span class="tag-remove" role="button" aria-label="Remove ' + escapeHtml(tag) + '">&times;</span>';
            el.querySelector('.tag-remove').addEventListener('click', () => removeInterest(tag));
            dom.interestTags.appendChild(el);
        });
    }

    if (dom.interestInput) {
        dom.interestInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                addInterest(dom.interestInput.value);
                dom.interestInput.value = '';
            }
            if (e.key === 'Backspace' && dom.interestInput.value === '' && interests.length > 0) {
                removeInterest(interests[interests.length - 1]);
            }
        });

        const interestWrapper = dom.interestInput.closest('.interest-input-wrapper');
        if (interestWrapper) {
            interestWrapper.addEventListener('click', () => {
                dom.interestInput.focus();
            });
        }
    }

    // ===================================================================
    //  Media Stream (Camera & Microphone) Management
    // ===================================================================

    async function initLocalMediaStream() {
        if (localStream) {
            // Already initialized, ensure tracks are enabled
            return localStream;
        }

        hideMediaAlert();

        const constraints = {
            video: {
                width: { ideal: 1280 },
                height: { ideal: 720 },
                facingMode: currentFacingMode,
            },
            audio: {
                echoCancellation: true,
                noiseSuppression: true,
                autoGainControl: true,
            },
        };

        try {
            localStream = await navigator.mediaDevices.getUserMedia(constraints);
            dom.localVideo.srcObject = localStream;
            dom.localVideo.play().catch((err) => console.warn('Local video play error:', err));
            dom.localPlaceholder.classList.add('hidden');
            isVideoEnabled = true;
            isAudioEnabled = true;
            updateMediaButtonStates();
            return localStream;
        } catch (err) {
            console.warn('Advanced camera constraints failed, attempting fallback constraints:', err);
            try {
                localStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
                dom.localVideo.srcObject = localStream;
                dom.localVideo.play().catch((err) => console.warn('Local video play error:', err));
                dom.localPlaceholder.classList.add('hidden');
                isVideoEnabled = true;
                isAudioEnabled = true;
                updateMediaButtonStates();
                return localStream;
            } catch (fallbackErr) {
                console.error('Unable to access camera and microphone:', fallbackErr);
                showMediaAlert(
                    'Camera & Microphone Access Required',
                    'Could not access camera or microphone. Please allow permissions in your browser, or switch to Text chat.'
                );
                throw fallbackErr;
            }
        }
    }

    function toggleAudio() {
        if (!localStream) return;
        isAudioEnabled = !isAudioEnabled;
        localStream.getAudioTracks().forEach((track) => {
            track.enabled = isAudioEnabled;
        });

        updateMediaButtonStates();
        sendMediaToggleSignal();
    }

    function toggleVideo() {
        if (!localStream) return;
        isVideoEnabled = !isVideoEnabled;
        localStream.getVideoTracks().forEach((track) => {
            track.enabled = isVideoEnabled;
        });

        if (isVideoEnabled) {
            dom.localPlaceholder.classList.add('hidden');
        } else {
            dom.localPlaceholder.classList.remove('hidden');
        }

        updateMediaButtonStates();
        sendMediaToggleSignal();
    }

    async function flipCamera() {
        if (!localStream || mode !== Mode.VIDEO) return;

        currentFacingMode = currentFacingMode === 'user' ? 'environment' : 'user';

        // Stop current video track
        localStream.getVideoTracks().forEach((t) => t.stop());

        try {
            const newStream = await navigator.mediaDevices.getUserMedia({
                video: { facingMode: currentFacingMode },
                audio: false,
            });

            const newVideoTrack = newStream.getVideoTracks()[0];
            if (newVideoTrack) {
                // Replace track in localStream
                const oldTracks = localStream.getVideoTracks();
                if (oldTracks.length > 0) {
                    localStream.removeTrack(oldTracks[0]);
                }
                localStream.addTrack(newVideoTrack);

                // Replace track on peer connection
                if (pc) {
                    const senders = pc.getSenders();
                    const videoSender = senders.find((s) => s.track && s.track.kind === 'video');
                    if (videoSender) {
                        videoSender.replaceTrack(newVideoTrack);
                    }
                }

                dom.localVideo.srcObject = localStream;
                dom.localVideo.style.transform = currentFacingMode === 'user' ? 'scaleX(-1)' : 'none';
            }
        } catch (err) {
            console.error('Failed to flip camera:', err);
        }
    }

    function updateMediaButtonStates() {
        // Audio button
        if (dom.toggleMicBtn) {
            const micOn = dom.toggleMicBtn.querySelector('.icon-on');
            const micOff = dom.toggleMicBtn.querySelector('.icon-off');
            if (isAudioEnabled) {
                dom.toggleMicBtn.classList.remove('btn-muted');
                if (micOn) micOn.classList.remove('hidden');
                if (micOff) micOff.classList.add('hidden');
                dom.toggleMicBtn.title = 'Mute Microphone (M)';
            } else {
                dom.toggleMicBtn.classList.add('btn-muted');
                if (micOn) micOn.classList.add('hidden');
                if (micOff) micOff.classList.remove('hidden');
                dom.toggleMicBtn.title = 'Unmute Microphone (M)';
            }
        }

        // Video button
        if (dom.toggleCamBtn) {
            const camOn = dom.toggleCamBtn.querySelector('.icon-on');
            const camOff = dom.toggleCamBtn.querySelector('.icon-off');
            if (isVideoEnabled) {
                dom.toggleCamBtn.classList.remove('btn-muted');
                if (camOn) camOn.classList.remove('hidden');
                if (camOff) camOff.classList.add('hidden');
                dom.toggleCamBtn.title = 'Turn Off Camera (V)';
            } else {
                dom.toggleCamBtn.classList.add('btn-muted');
                if (camOn) camOn.classList.add('hidden');
                if (camOff) camOff.classList.remove('hidden');
                dom.toggleCamBtn.title = 'Turn On Camera (V)';
            }
        }
    }

    function sendMediaToggleSignal() {
        if (state === State.CHATTING && mode === Mode.VIDEO) {
            sendMessage({
                type: 'media_toggle',
                mediaState: {
                    video: isVideoEnabled,
                    audio: isAudioEnabled,
                },
            });
        }
    }

    function stopAllLocalMedia() {
        if (localStream) {
            localStream.getTracks().forEach((track) => track.stop());
            localStream = null;
        }
        if (dom.localVideo) {
            dom.localVideo.srcObject = null;
        }
        dom.localPlaceholder.classList.add('hidden');
    }

    function showMediaAlert(title, message) {
        dom.mediaAlertTitle.textContent = title;
        dom.mediaAlertMsg.textContent = message;
        dom.mediaAlert.classList.remove('hidden');
    }

    function hideMediaAlert() {
        dom.mediaAlert.classList.add('hidden');
    }

    // ===================================================================
    //  WebRTC PeerConnection Management
    // ===================================================================

    function ensurePeerConnection() {
        if (!pc) {
            createPeerConnection();
        }
        return pc;
    }

    function createPeerConnection() {
        if (pc) {
            pc.close();
            pc = null;
        }

        console.log('[WebRTC] Creating RTCPeerConnection with STUN servers');
        pc = new RTCPeerConnection(rtcConfig);
        remoteDescriptionSet = false;

        // Attach local audio/video tracks
        if (localStream) {
            localStream.getTracks().forEach((track) => {
                console.log('[WebRTC] Adding local track to PC:', track.kind, track.label);
                pc.addTrack(track, localStream);
            });
        }

        // Handle incoming remote tracks from stranger (supports both streams[0] and individual tracks)
        pc.ontrack = (event) => {
            console.log('[WebRTC] ontrack received:', event);
            let stream = null;
            if (event.streams && event.streams[0]) {
                stream = event.streams[0];
            } else if (event.track) {
                if (!dom.remoteVideo.srcObject) {
                    dom.remoteVideo.srcObject = new MediaStream();
                }
                dom.remoteVideo.srcObject.addTrack(event.track);
                stream = dom.remoteVideo.srcObject;
            }

            if (stream) {
                dom.remoteVideo.srcObject = stream;
                dom.remoteVideo.play().catch((err) => console.warn('Remote video play error:', err));
                dom.remotePlaceholder.classList.add('hidden');
                dom.remoteConnStatus.className = 'conn-status-indicator live';
            }
        };

        // Trickle ICE Candidates (only send valid candidates)
        pc.onicecandidate = (event) => {
            if (event.candidate && event.candidate.candidate) {
                sendMessage({
                    type: 'webrtc_candidate',
                    candidate: event.candidate,
                });
            }
        };

        // Peer connection state changes
        pc.onconnectionstatechange = () => {
            if (!pc) return;
            console.log('[WebRTC] Connection state change:', pc.connectionState);
            switch (pc.connectionState) {
                case 'connected':
                    dom.remoteConnStatus.className = 'conn-status-indicator live';
                    dom.remotePlaceholder.classList.add('hidden');
                    break;
                case 'connecting':
                    dom.remoteConnStatus.className = 'conn-status-indicator connecting';
                    break;
                case 'disconnected':
                case 'failed':
                case 'closed':
                    dom.remoteConnStatus.className = 'conn-status-indicator';
                    break;
            }
        };

        pc.oniceconnectionstatechange = () => {
            if (!pc) return;
            console.log('[WebRTC] ICE connection state:', pc.iceConnectionState);
            if (pc.iceConnectionState === 'connected' || pc.iceConnectionState === 'completed') {
                dom.remotePlaceholder.classList.add('hidden');
                dom.remoteConnStatus.className = 'conn-status-indicator live';
            }
        };
    }

    async function startWebRTCCall() {
        if (mode !== Mode.VIDEO) return;

        try {
            ensurePeerConnection();

            if (isInitiator) {
                console.log('[WebRTC] Acting as initiator: creating offer...');
                const offer = await pc.createOffer({
                    offerToReceiveAudio: true,
                    offerToReceiveVideo: true,
                });
                await pc.setLocalDescription(offer);

                sendMessage({
                    type: 'webrtc_offer',
                    sdp: offer.sdp,
                });
            } else {
                console.log('[WebRTC] Acting as receiver: waiting for remote offer...');
            }
        } catch (err) {
            console.error('[WebRTC] Error starting call:', err);
        }
    }

    async function handleRemoteOffer(msg) {
        if (mode !== Mode.VIDEO) return;

        try {
            console.log('[WebRTC] Received remote offer, creating answer...');
            ensurePeerConnection();
            await pc.setRemoteDescription(new RTCSessionDescription({ type: 'offer', sdp: msg.sdp }));
            remoteDescriptionSet = true;
            await flushPendingCandidates();

            const answer = await pc.createAnswer();
            await pc.setLocalDescription(answer);

            sendMessage({
                type: 'webrtc_answer',
                sdp: answer.sdp,
            });

            // Send initial media mute states to peer
            sendMediaToggleSignal();
        } catch (err) {
            console.error('[WebRTC] Error handling remote offer:', err);
        }
    }

    async function handleRemoteAnswer(msg) {
        if (mode !== Mode.VIDEO || !pc) return;

        try {
            console.log('[WebRTC] Received remote answer, setting remote description...');
            if (pc.signalingState !== 'stable') {
                await pc.setRemoteDescription(new RTCSessionDescription({ type: 'answer', sdp: msg.sdp }));
                remoteDescriptionSet = true;
                await flushPendingCandidates();

                // Send initial media mute states to peer
                sendMediaToggleSignal();
            }
        } catch (err) {
            console.error('[WebRTC] Error handling remote answer:', err);
        }
    }

    async function handleRemoteCandidate(msg) {
        if (mode !== Mode.VIDEO || !msg.candidate) return;

        try {
            const candidate = new RTCIceCandidate(msg.candidate);
            if (pc && remoteDescriptionSet) {
                await pc.addIceCandidate(candidate);
            } else {
                pendingRemoteCandidates.push(candidate);
            }
        } catch (err) {
            console.error('[WebRTC] Error adding ICE candidate:', err);
        }
    }

    async function flushPendingCandidates() {
        if (!pc || !remoteDescriptionSet) return;
        while (pendingRemoteCandidates.length > 0) {
            const candidate = pendingRemoteCandidates.shift();
            try {
                await pc.addIceCandidate(candidate);
            } catch (err) {
                console.error('[WebRTC] Error adding queued ICE candidate:', err);
            }
        }
    }

    function handleMediaToggle(msg) {
        if (!msg.mediaState) return;

        if (msg.mediaState.video === false) {
            dom.remoteCamOff.classList.remove('hidden');
        } else {
            dom.remoteCamOff.classList.add('hidden');
        }

        if (msg.mediaState.audio === false) {
            dom.remoteMicOff.classList.remove('hidden');
        } else {
            dom.remoteMicOff.classList.add('hidden');
        }
    }

    function cleanupPeerConnection() {
        if (pc) {
            pc.ontrack = null;
            pc.onicecandidate = null;
            pc.onconnectionstatechange = null;
            pc.oniceconnectionstatechange = null;
            pc.close();
            pc = null;
        }

        if (dom.remoteVideo) {
            dom.remoteVideo.srcObject = null;
        }

        dom.remotePlaceholder.classList.remove('hidden');
        if (dom.remotePlaceholderText) {
            if (state === State.SEARCHING) {
                dom.remotePlaceholderText.textContent = 'Looking for a stranger...';
            } else if (state === State.DISCONNECTED) {
                dom.remotePlaceholderText.textContent = 'Stranger has disconnected';
            } else {
                dom.remotePlaceholderText.textContent = 'Looking for a stranger...';
            }
        }
        dom.remoteConnStatus.className = 'conn-status-indicator';
        dom.remoteCamOff.classList.add('hidden');
        dom.remoteMicOff.classList.add('hidden');
        pendingRemoteCandidates = [];
        remoteDescriptionSet = false;
    }

    // ===================================================================
    //  WebSocket Connection
    // ===================================================================

    async function startChat(chosenMode) {
        mode = chosenMode;

        // If video mode, ensure local media is ready before connecting
        if (mode === Mode.VIDEO) {
            try {
                await initLocalMediaStream();
            } catch (err) {
                // Media failed and alert is shown, abort
                return;
            }
        }

        connectWebSocket();
    }

    function connectWebSocket() {
        if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
            ws.close();
        }

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.host;
        const params = new URLSearchParams();

        params.set('mode', mode);
        if (interests.length > 0) {
            params.set('interests', interests.join(','));
        }

        const url = protocol + '//' + host + '/ws?' + params.toString();

        ws = new WebSocket(url);
        ws.addEventListener('open', onOpen);
        ws.addEventListener('message', onMessage);
        ws.addEventListener('close', onClose);
        ws.addEventListener('error', onError);
    }

    function disconnectWebSocket() {
        if (ws) {
            ws.close();
            ws = null;
        }
    }

    function sendMessage(msg) {
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify(msg));
        }
    }

    // ===================================================================
    //  WebSocket Handlers
    // ===================================================================

    function onOpen() {
        setState(State.SEARCHING);
    }

    function onMessage(event) {
        let msg;
        try {
            msg = JSON.parse(event.data);
        } catch (e) {
            return;
        }

        switch (msg.type) {
            case 'system':
                handleSystemMessage(msg);
                break;
            case 'chat':
                handleChatMessage(msg);
                break;
            case 'typing':
                showTypingIndicator(true);
                break;
            case 'stop_typing':
                showTypingIndicator(false);
                break;
            case 'webrtc_offer':
                handleRemoteOffer(msg);
                break;
            case 'webrtc_answer':
                handleRemoteAnswer(msg);
                break;
            case 'webrtc_candidate':
                handleRemoteCandidate(msg);
                break;
            case 'media_toggle':
                handleMediaToggle(msg);
                break;
        }
    }

    function onClose() {
        cleanupPeerConnection();
        if (state === State.CHATTING || state === State.SEARCHING) {
            setState(State.DISCONNECTED);
        }
    }

    function onError() {
        appendSystemMessage('Connection error. Please try again.');
    }

    // ===================================================================
    //  Message Handling
    // ===================================================================

    function handleSystemMessage(msg) {
        const content = msg.content || '';

        if (content.includes("You're now chatting")) {
            isInitiator = !!msg.isInitiator;
            setState(State.CHATTING);
            appendSystemMessage(content, 'matched');

            if (msg.commonInterests && msg.commonInterests.length > 0) {
                showCommonInterests(msg.commonInterests);
            }

            // Start WebRTC handshake if in video mode
            if (mode === Mode.VIDEO) {
                startWebRTCCall();
            }
        } else if (content.includes('disconnected')) {
            cleanupPeerConnection();
            appendSystemMessage(content, 'disconnected');
            setState(State.DISCONNECTED);
            showTypingIndicator(false);
        } else {
            appendSystemMessage(content);
        }
    }

    function handleChatMessage(msg) {
        showTypingIndicator(false);
        appendChatMessage('Stranger', msg.content);
    }

    // ===================================================================
    //  State Machine & Layout Controller
    // ===================================================================

    function setState(newState) {
        state = newState;
        resetDisconnectConfirmation();

        // Update mode badge in header
        if (state !== State.IDLE) {
            dom.modeBadge.classList.remove('hidden');
            dom.modeBadgeText.textContent = mode === Mode.VIDEO ? 'Video Chat' : 'Text Chat';
            if (mode === Mode.TEXT) {
                dom.modeBadge.classList.add('text-mode-active');
            } else {
                dom.modeBadge.classList.remove('text-mode-active');
            }
        } else {
            dom.modeBadge.classList.add('hidden');
        }

        // Update layout structure
        if (mode === Mode.TEXT) {
            dom.chatLayout.classList.add('text-only-mode');
        } else {
            dom.chatLayout.classList.remove('text-only-mode');
        }

        switch (state) {
            case State.IDLE:
                dom.landingPanel.classList.remove('hidden');
                dom.chatPanel.classList.add('hidden');
                setMessageInputEnabled(false);
                cleanupPeerConnection();
                stopAllLocalMedia();
                break;

            case State.SEARCHING:
                dom.landingPanel.classList.add('hidden');
                dom.chatPanel.classList.remove('hidden');
                dom.chatLog.innerHTML = '';
                hideCommonInterests();
                showTypingIndicator(false);
                setStatus('Looking for a stranger...', 'searching');
                if (dom.remotePlaceholderText) {
                    dom.remotePlaceholderText.textContent = 'Looking for a stranger...';
                }
                dom.remotePlaceholder.classList.remove('hidden');
                setMessageInputEnabled(false);
                dom.disconnectBtn.classList.remove('hidden');
                dom.newBtn.classList.add('hidden');
                dom.disconnectBtnLabel.textContent = 'Disconnect';
                cleanupPeerConnection();
                break;

            case State.CHATTING:
                setStatus("Connected with a random stranger! Say hi.", 'connected');
                if (dom.remotePlaceholderText && mode === Mode.VIDEO) {
                    dom.remotePlaceholderText.textContent = 'Connecting video...';
                }
                setMessageInputEnabled(true);
                dom.messageInput.focus();
                dom.disconnectBtn.classList.remove('hidden');
                dom.newBtn.classList.add('hidden');
                dom.disconnectBtnLabel.textContent = 'Disconnect';
                break;

            case State.DISCONNECTED:
                setStatus('Stranger has disconnected.', 'disconnected');
                if (dom.remotePlaceholderText) {
                    dom.remotePlaceholderText.textContent = 'Stranger has disconnected';
                }
                dom.remotePlaceholder.classList.remove('hidden');
                setMessageInputEnabled(false);
                showTypingIndicator(false);
                dom.disconnectBtn.classList.add('hidden');
                dom.newBtn.classList.remove('hidden');
                cleanupPeerConnection();
                break;
        }
    }

    // ===================================================================
    //  UI Helpers
    // ===================================================================

    function setStatus(text, className) {
        dom.statusText.textContent = text;
        dom.statusText.className = 'status-text' + (className ? ' ' + className : '');
    }

    function setMessageInputEnabled(enabled) {
        dom.messageInput.disabled = !enabled;
        dom.sendBtn.disabled = !enabled;
        if (!enabled) {
            dom.messageInput.value = '';
        }
    }

    function appendChatMessage(sender, content) {
        const div = document.createElement('div');
        div.className = 'chat-message';
        const senderClass = sender === 'You' ? 'you' : 'stranger';
        div.innerHTML =
            '<span class="msg-sender ' + senderClass + '">' + sender + ':</span> ' +
            '<span class="msg-content">' + escapeHtml(content) + '</span>';
        dom.chatLog.appendChild(div);
        scrollToBottom();
    }

    function appendSystemMessage(content, modifier) {
        const div = document.createElement('div');
        div.className = 'chat-message system-msg' + (modifier ? ' ' + modifier : '');
        div.textContent = content;
        dom.chatLog.appendChild(div);
        scrollToBottom();
    }

    function showCommonInterests(commonList) {
        dom.commonInterestsList.textContent = commonList.join(', ');
        dom.commonInterests.classList.remove('hidden');
    }

    function hideCommonInterests() {
        dom.commonInterests.classList.add('hidden');
        dom.commonInterestsList.textContent = '';
    }

    function showTypingIndicator(show) {
        if (show) {
            dom.typingIndicator.classList.remove('hidden');
            scrollToBottom();
        } else {
            dom.typingIndicator.classList.add('hidden');
        }
    }

    function scrollToBottom() {
        requestAnimationFrame(() => {
            dom.chatLog.scrollTop = dom.chatLog.scrollHeight;
        });
    }

    function escapeHtml(str) {
        const div = document.createElement('div');
        div.appendChild(document.createTextNode(str));
        return div.innerHTML;
    }

    // ===================================================================
    //  Typing Indicators
    // ===================================================================

    function handleTyping() {
        if (state !== State.CHATTING) return;

        if (!isTyping) {
            isTyping = true;
            sendMessage({ type: 'typing' });
        }

        clearTimeout(typingTimeout);
        typingTimeout = setTimeout(() => {
            isTyping = false;
            sendMessage({ type: 'stop_typing' });
        }, 1500);
    }

    function resetTypingState() {
        clearTimeout(typingTimeout);
        if (isTyping) {
            isTyping = false;
        }
    }

    // ===================================================================
    //  Disconnect / Omegle 2-Step Confirmation
    // ===================================================================

    function handleDisconnectClick() {
        if (state === State.CHATTING) {
            if (!isConfirmingDisconnect) {
                // Step 1: Prompt "Really? (Esc)"
                isConfirmingDisconnect = true;
                dom.disconnectBtnLabel.textContent = 'Really?';
                clearTimeout(disconnectConfirmTimer);
                disconnectConfirmTimer = setTimeout(resetDisconnectConfirmation, 2500);
            } else {
                // Step 2: Confirmed!
                resetDisconnectConfirmation();
                sendMessage({ type: 'disconnect' });
                cleanupPeerConnection();
                appendSystemMessage('You have disconnected.', 'disconnected');
                setState(State.DISCONNECTED);
                resetTypingState();
            }
        } else if (state === State.SEARCHING) {
            sendMessage({ type: 'leave_queue' });
            disconnectWebSocket();
            setState(State.IDLE);
        }
    }

    function resetDisconnectConfirmation() {
        isConfirmingDisconnect = false;
        clearTimeout(disconnectConfirmTimer);
        if (dom.disconnectBtnLabel) {
            dom.disconnectBtnLabel.textContent = 'Disconnect';
        }
    }

    function handleNewStrangerClick() {
        cleanupPeerConnection();
        dom.chatLog.innerHTML = '';
        setState(State.SEARCHING);
        if (ws && ws.readyState === WebSocket.OPEN) {
            sendMessage({ type: 'find_match' });
        } else {
            connectWebSocket();
        }
    }

    function handleLeaveHomeClick() {
        cleanupPeerConnection();
        stopAllLocalMedia();
        if (ws && ws.readyState === WebSocket.OPEN) {
            sendMessage({ type: 'leave_queue' });
        }
        disconnectWebSocket();
        setState(State.IDLE);
    }

    // ===================================================================
    //  Message Sending
    // ===================================================================

    function doSend() {
        const content = dom.messageInput.value.trim();
        if (!content || state !== State.CHATTING) return;

        sendMessage({ type: 'chat', content: content });
        appendChatMessage('You', content);
        dom.messageInput.value = '';
        resetTypingState();
    }

    // ===================================================================
    //  Online Count Polling
    // ===================================================================

    function startStatsPolling() {
        fetchStats();
        statsInterval = setInterval(fetchStats, 5000);
    }

    function fetchStats() {
        fetch('/api/stats')
            .then((res) => res.json())
            .then((data) => {
                if (data && typeof data.online === 'number') {
                    dom.onlineCount.textContent = data.online.toLocaleString();
                }
            })
            .catch(() => {});
    }

    // ===================================================================
    //  Event Listeners
    // ===================================================================

    // Landing start buttons
    if (dom.startVideoBtn) {
        dom.startVideoBtn.addEventListener('click', () => {
            console.log('[GOmegle] Start Video Chat clicked');
            startChat(Mode.VIDEO);
        });
    }

    if (dom.startTextBtn) {
        dom.startTextBtn.addEventListener('click', () => {
            console.log('[GOmegle] Start Text Chat clicked');
            startChat(Mode.TEXT);
        });
    }

    // Fallback start button for any cached markup
    if (dom.startBtn) {
        dom.startBtn.addEventListener('click', () => {
            console.log('[GOmegle] Fallback start button clicked');
            startChat(Mode.VIDEO);
        });
    }

    // Chat controls
    if (dom.sendBtn) {
        dom.sendBtn.addEventListener('click', doSend);
    }

    if (dom.messageInput) {
        dom.messageInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                doSend();
            }
        });
        dom.messageInput.addEventListener('input', handleTyping);
    }

    if (dom.disconnectBtn) {
        dom.disconnectBtn.addEventListener('click', handleDisconnectClick);
    }
    if (dom.newBtn) {
        dom.newBtn.addEventListener('click', handleNewStrangerClick);
    }
    if (dom.leaveHomeBtn) {
        dom.leaveHomeBtn.addEventListener('click', handleLeaveHomeClick);
    }

    // Media toggle controls
    if (dom.toggleMicBtn) {
        dom.toggleMicBtn.addEventListener('click', toggleAudio);
    }
    if (dom.toggleCamBtn) {
        dom.toggleCamBtn.addEventListener('click', toggleVideo);
    }
    if (dom.flipCamBtn) {
        dom.flipCamBtn.addEventListener('click', flipCamera);
    }

    // Media alert action buttons
    if (dom.mediaRetryBtn) {
        dom.mediaRetryBtn.addEventListener('click', () => {
            startChat(Mode.VIDEO);
        });
    }

    if (dom.mediaFallbackTextBtn) {
        dom.mediaFallbackTextBtn.addEventListener('click', () => {
            hideMediaAlert();
            startChat(Mode.TEXT);
        });
    }

    if (dom.mediaDismissBtn) {
        dom.mediaDismissBtn.addEventListener('click', hideMediaAlert);
    }

    // Keyboard Shortcuts
    document.addEventListener('keydown', (e) => {
        // Ignore shortcuts when user is typing in text inputs
        const isInputFocused =
            document.activeElement === dom.messageInput ||
            document.activeElement === dom.interestInput;

        if (e.key === 'Escape') {
            e.preventDefault();
            if (state === State.CHATTING) {
                handleDisconnectClick();
            } else if (state === State.DISCONNECTED) {
                handleNewStrangerClick();
            } else if (state === State.SEARCHING) {
                handleLeaveHomeClick();
            }
        }

        // Quick mute hotkeys when not typing in chat input
        if (!isInputFocused && state === State.CHATTING && mode === Mode.VIDEO) {
            if (e.key === 'm' || e.key === 'M') {
                toggleAudio();
            } else if (e.key === 'v' || e.key === 'V') {
                toggleVideo();
            }
        }
    });

    // Page unload safety: cleanly stop tracks
    window.addEventListener('beforeunload', () => {
        stopAllLocalMedia();
        cleanupPeerConnection();
        disconnectWebSocket();
    });

    // ===================================================================
    //  Initialization
    // ===================================================================

    startStatsPolling();
})();
