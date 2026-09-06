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
        // Home: a press moves the event between Mine and Upcoming, so take
        // both columns from the response instead of patching one form.
        var columns = document.querySelector('.columns'), freshColumns = doc.querySelector('.columns');
        if (columns && freshColumns) {
          columns.replaceWith(freshColumns);
          // The pressed button was inside the subtree we just discarded, so
          // focus would fall to <body> and a keyboard visitor would restart
          // from the top of the page. Put it back on the same control, and
          // let its aria-pressed announce the new state on the way.
          var again = freshColumns.querySelector('form[action="' + key + '"] [aria-pressed]');
          if (again) again.focus({ preventScroll: true });
          return;
        }
        var fresh = doc.querySelector('form[action="' + key + '"]');
        var row = form.closest('.events li'), series = row && row.dataset.series;
        if (fresh) {
          form.replaceWith(fresh);
        } else {
          // No matching form in the new page: the event left this list (hidden,
          // or unfollowed from Mine). Take the row with it; otherwise just unlock.
          if (row) row.remove(); else delete form.dataset.busy;
        }
        if (!series) return;
        // Follow and hide apply to every date of a repeating event, so bring
        // its other rows up to date from the same response.
        document.querySelectorAll('.events li[data-series="' + series + '"]').forEach(function (li) {
          if (li === row) return;
          var f = li.querySelector('form[data-toggle]');
          var nf = f && doc.querySelector('form[action="' + f.getAttribute('action') + '"]');
          if (nf) f.replaceWith(nf); else li.remove();
        });
      })
      // Hand back to the browser. Unlock first: if that submission never
      // navigates, the control has to stay pressable rather than dead.
      .catch(function () {
        delete form.dataset.busy;
        form.dataset.native = '1';
        btn.click();
      });
  });


  // Theme toggle: the server cannot see the system setting, so when no choice
  // is stored it offers "dark"; flip the offer if the system is already dark.
  var t = document.querySelector('form.theme button[name="theme"]');
  if (t && !document.documentElement.hasAttribute('data-theme') &&
      window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
    t.value = 'light'; t.textContent = 'light';
    t.setAttribute('aria-label', 'Switch to light theme');
  }

  if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js').catch(function () {});
})();
