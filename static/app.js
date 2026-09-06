// dside events — progressive enhancement only. Every form works without this file.
(function () {
  'use strict';

  // Two forms are "the same" when action, kind and key match (several /follow forms share a page).
  function same(a, b) {
    return ['kind', 'key'].every(function (n) {
      var x = a.elements[n], y = b.elements[n];
      return (x ? x.value : '') === (y ? y.value : '');
    });
  }

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
        var fresh = Array.prototype.filter.call(
          doc.querySelectorAll('form[action="' + key + '"]'),
          function (f) { return same(form, f); }
        )[0];
        if (fresh) form.replaceWith(fresh); else delete form.dataset.busy;
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
