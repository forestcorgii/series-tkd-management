// Series Taekwondo Management System (STMS) - Web Push Client Engine
(function () {
    'use strict';

    function urlBase64ToUint8Array(base64String) {
        const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
        const base64 = (base64String + padding)
            .replace(/\-/g, '+')
            .replace(/_/g, '/');

        const rawData = window.atob(base64);
        const outputArray = new Uint8Array(rawData.length);

        for (let i = 0; i < rawData.length; ++i) {
            outputArray[i] = rawData.charCodeAt(i);
        }
        return outputArray;
    }

    const STMSPush = {
        isSupported: function () {
            return (
                'serviceWorker' in navigator &&
                'PushManager' in window &&
                'Notification' in window
            );
        },

        getPermissionState: function () {
            if (!this.isSupported()) return 'unsupported';
            return Notification.permission;
        },

        getReadyRegistration: async function () {
            if (!this.isSupported()) return null;
            return await navigator.serviceWorker.ready;
        },

        getSubscription: async function () {
            const reg = await this.getReadyRegistration();
            if (!reg) return null;
            return await reg.pushManager.getSubscription();
        },

        subscribe: async function () {
            if (!this.isSupported()) {
                throw new Error('Web Push is not supported by this browser or device.');
            }

            // 1. Request OS/Browser Notification Permission
            const permission = await Notification.requestPermission();
            if (permission !== 'granted') {
                return { success: false, permission: permission, reason: 'permission_denied' };
            }

            // 2. Fetch server's active VAPID Public Key
            const keyRes = await fetch('/api/push/vapid-public-key');
            if (!keyRes.ok) {
                throw new Error('Failed to fetch VAPID public key from server');
            }
            const keyData = await keyRes.json();
            if (!keyData.public_key) {
                throw new Error('Server returned empty VAPID public key');
            }

            // 3. Register subscription with browser push service
            const reg = await this.getReadyRegistration();
            if (!reg) {
                throw new Error('Service worker is not active');
            }

            let sub = await reg.pushManager.getSubscription();
            if (!sub) {
                sub = await reg.pushManager.subscribe({
                    userVisibleOnly: true,
                    applicationServerKey: urlBase64ToUint8Array(keyData.public_key)
                });
            }

            // 4. Send subscription to STMS backend
            const subJSON = sub.toJSON();
            const saveRes = await fetch('/api/push/subscribe', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(subJSON)
            });

            if (!saveRes.ok) {
                const errData = await saveRes.json().catch(() => ({}));
                throw new Error(errData.error || 'Failed to register subscription on server');
            }

            return { success: true, permission: 'granted', subscription: sub };
        },

        unsubscribe: async function () {
            const reg = await this.getReadyRegistration();
            if (!reg) return { success: true };

            const sub = await reg.pushManager.getSubscription();
            if (sub) {
                const endpoint = sub.endpoint;
                await sub.unsubscribe();

                await fetch('/api/push/unsubscribe', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ endpoint: endpoint })
                }).catch(() => {});
            }

            return { success: true };
        },

        sendTestPush: async function () {
            const res = await fetch('/api/push/test', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' }
            });

            const data = await res.json().catch(() => ({}));
            if (!res.ok) {
                throw new Error(data.error || 'Failed to dispatch test push notification');
            }
            return data;
        }
    };

    window.STMSPush = STMSPush;

    // Auto-sync existing subscription on page load if permission is already granted
    window.addEventListener('load', async () => {
        try {
            if (STMSPush.isSupported() && Notification.permission === 'granted') {
                const reg = await STMSPush.getReadyRegistration();
                if (reg) {
                    const sub = await reg.pushManager.getSubscription();
                    if (sub) {
                        fetch('/api/push/subscribe', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify(sub.toJSON())
                        }).catch(() => {});
                    }
                }
            }
        } catch (e) {
            // Non-critical auto-sync failure
        }
    });
})();
