// dside events — progressive enhancement only. Every form works without this file.
(function () {
  'use strict';

  document.addEventListener('submit', function (e) {
    var form = e.target, btn = e.submitter;
    if (!form.matches('form[data-toggle]') || form.dataset.native || !btn || !window.fetch) return;
    if (form.dataset.busy) { e.preventDefault(); return; }
    e.preventDefault();
    form.dataset.busy = '1';

    var wasPressed = btn.getAttribute('aria-pressed') === 'true';
    form.querySelectorAll('[aria-pressed]').forEach(function (b) {
      var on = b === btn && !wasPressed;
      b.setAttribute('aria-pressed', on ? 'true' : 'false');
      // The label names the action while unpressed and the state once pressed.
      if (b.dataset.on && b.dataset.off) b.textContent = on ? b.dataset.on : b.dataset.off;
    });

    var body = new FormData(form);
    if (btn.name) body.append(btn.name, btn.value);
    var key = form.getAttribute('action');

    fetch(form.action, { method: 'POST', body: body, credentials: 'same-origin' })
      .then(function (r) {
        if (!r.ok) throw new Error(r.status);
        return r.text();
      })
      .then(function (html) {
        if (!html) return;
        var doc = new DOMParser().parseFromString(html, 'text/html');
        var fresh = doc.querySelector('form[action="' + key + '"]');
        if (fresh) { form.replaceWith(fresh); return; }
        // No matching form in the new page: the event left this list (hidden,
        // or unfollowed from Mine). Take the row with it; otherwise just unlock.
        var row = form.closest('.events li');
        if (row) row.remove(); else delete form.dataset.busy;
      })
      .catch(function () { form.dataset.native = '1'; btn.click(); });
  });


  // Theme toggle: the server cannot see the system setting, so when no choice
  // is stored it offers "dark"; flip the offer if the system is already dark.
  var t = document.querySelector('form.theme button[name="theme"]');
  if (t && !document.documentElement.hasAttribute('data-theme') &&
      window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
    t.value = 'light'; t.textContent = 'light';
  }

  if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js').catch(function () {});
})();
