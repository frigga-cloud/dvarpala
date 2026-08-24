/*
 * Dvarpala captive portal.
 *
 * The sign-in forms are ordinary HTML forms and work with no JavaScript at
 * all - which matters, because this page is often opened by the small browser
 * an operating system pops up when it detects a captive portal, and those are
 * not full browsers.
 *
 * All this adds is noticing that a sign-in finished somewhere else: a person
 * who completes the flow in another tab should not be left staring at a page
 * still asking them to log in.
 */
(function () {
    'use strict';

    var POLL_MS = 5000;

    function checkStatus() {
        fetch('/api/internal/auth-status', { credentials: 'same-origin' })
            .then(function (r) { return r.ok ? r.json() : null; })
            .then(function (data) {
                if (data && data.authenticated) {
                    window.location.href = '/auth/success';
                    return;
                }
                window.setTimeout(checkStatus, POLL_MS);
            })
            .catch(function () {
                // The portal being briefly unreachable is not worth reporting:
                // the person is mid-signin and can see the page in front of
                // them. Try again rather than showing an alarming message.
                window.setTimeout(checkStatus, POLL_MS);
            });
    }

    window.setTimeout(checkStatus, POLL_MS);
})();
