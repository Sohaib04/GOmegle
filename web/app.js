/* ===================================================================
   GOmegle — Client Application
   WebSocket chat client with state machine, typing detection,
   interest tags, and online count polling.
   =================================================================== */

(function () {
    'use strict';

    // --- State Machine ---
    const State = {
        IDLE: 'idle',
        SEARCHING: 'searching',
        CHATTING: 'chatting',
        DISCONNECTED: 'disconnected',
    };

    // --- App State ---
    let state = State.IDLE;
    let ws = null;
    let interests = [];
    let typingTimeout = null;
    let isTyping = false;
    let statsInterval = null;

    // --- DOM References ---
    const $ = (id) => document.getElementById(id);

    const dom = {
        landingPanel: $('landing-panel'),
        chatPanel: $('chat-panel'),
        startBtn: $('start-btn'),
        interestInput: $('interest-input'),
        interestTags: $('interest-tags'),
        statusBar: $('status-bar'),
        statusText: $('status-text'),
        commonInterests: $('common-interests'),
        commonInterestsList: $('common-interests-list'),
        chatLog: $('chat-log'),
        typingIndicator: $('typing-indicator'),
        messageInput: $('message-input'),
        sendBtn: $('send-btn'),
        disconnectBtn: $('disconnect-btn'),
        newBtn: $('new-btn'),
        onlineCount: $('online-count'),
    };

    // ===================================================================
    //  Interest Tags
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

    // Click on wrapper focuses the input
    dom.interestInput.closest('.interest-input-wrapper').addEventListener('click', () => {
        dom.interestInput.focus();
    });

    // ===================================================================
    //  WebSocket Connection
    // ===================================================================

    function connect() {
        if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
            ws.close();
        }

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.host;
        const params = interests.length > 0 ? '?interests=' + encodeURIComponent(interests.join(',')) : '';
        const url = protocol + '//' + host + '/ws' + params;

        ws = new WebSocket(url);

        ws.addEventListener('open', onOpen);
        ws.addEventListener('message', onMessage);
        ws.addEventListener('close', onClose);
        ws.addEventListener('error', onError);
    }

    function disconnect() {
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
    //  WebSocket Event Handlers
    // ===================================================================

    function onOpen() {
        setState(State.SEARCHING);
    }

    function onMessage(event) {
        let msg;
        try {
            msg = JSON.parse(event.data);
        } catch {
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
        }
    }

    function onClose() {
        if (state === State.CHATTING || state === State.SEARCHING) {
            setState(State.DISCONNECTED);
        }
    }

    function onError() {
        appendSystemMessage('Connection error. Please try again.');
    }

    // ===================================================================
    //  Message Handlers
    // ===================================================================

    function handleSystemMessage(msg) {
        const content = msg.content || '';

        if (content.includes("You're now chatting")) {
            setState(State.CHATTING);
            appendSystemMessage(content, 'matched');

            // Show common interests if any
            if (msg.commonInterests && msg.commonInterests.length > 0) {
                showCommonInterests(msg.commonInterests);
            }
        } else if (content.includes('disconnected')) {
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
    //  State Machine
    // ===================================================================

    function setState(newState) {
        state = newState;

        switch (state) {
            case State.IDLE:
                dom.landingPanel.classList.remove('hidden');
                dom.chatPanel.classList.add('hidden');
                setMessageInputEnabled(false);
                break;

            case State.SEARCHING:
                dom.landingPanel.classList.add('hidden');
                dom.chatPanel.classList.remove('hidden');
                dom.chatLog.innerHTML = '';
                hideCommonInterests();
                showTypingIndicator(false);
                setStatus('Looking for a stranger...', 'searching');
                setMessageInputEnabled(false);
                dom.disconnectBtn.classList.remove('hidden');
                dom.newBtn.classList.add('hidden');
                break;

            case State.CHATTING:
                setStatus("You're now chatting with a random stranger!", 'connected');
                setMessageInputEnabled(true);
                dom.messageInput.focus();
                dom.disconnectBtn.classList.remove('hidden');
                dom.newBtn.classList.add('hidden');
                break;

            case State.DISCONNECTED:
                setStatus('Stranger has disconnected.', 'disconnected');
                setMessageInputEnabled(false);
                showTypingIndicator(false);
                dom.disconnectBtn.classList.add('hidden');
                dom.newBtn.classList.remove('hidden');
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
    //  Typing Detection
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
            .catch(() => {
                // Silently fail — stats are non-critical
            });
    }

    // ===================================================================
    //  Event Listeners
    // ===================================================================

    // Start button
    dom.startBtn.addEventListener('click', () => {
        connect();
    });

    // Send message
    function doSend() {
        const content = dom.messageInput.value.trim();
        if (!content || state !== State.CHATTING) return;

        sendMessage({ type: 'chat', content: content });
        appendChatMessage('You', content);
        dom.messageInput.value = '';
        resetTypingState();
    }

    dom.sendBtn.addEventListener('click', doSend);

    dom.messageInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            doSend();
        }
    });

    // Typing detection on input
    dom.messageInput.addEventListener('input', handleTyping);

    // Disconnect button
    dom.disconnectBtn.addEventListener('click', () => {
        if (state === State.CHATTING) {
            sendMessage({ type: 'next' });
            appendSystemMessage('You have disconnected.');
            setState(State.DISCONNECTED);
            resetTypingState();
        } else if (state === State.SEARCHING) {
            disconnect();
            setState(State.IDLE);
        }
    });

    // New chat button
    dom.newBtn.addEventListener('click', () => {
        disconnect();
        connect();
    });

    // Keyboard shortcut: Escape = disconnect/go back
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            if (state === State.CHATTING) {
                sendMessage({ type: 'next' });
                appendSystemMessage('You have disconnected.');
                setState(State.DISCONNECTED);
                resetTypingState();
            } else if (state === State.SEARCHING) {
                disconnect();
                setState(State.IDLE);
            } else if (state === State.DISCONNECTED) {
                disconnect();
                setState(State.IDLE);
            }
        }
    });

    // ===================================================================
    //  Init
    // ===================================================================

    startStatsPolling();
})();
