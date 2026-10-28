// Search and tag filters for the catalogue (WEB-4), copy buttons (WEB-5),
// score box (WEB-19) and one-click claim (WEB-20). The only host it calls is the data-api one (WEB-21).
(function () {
  var cards = Array.prototype.slice.call(document.querySelectorAll("#cards .card"));
  var q = document.getElementById("q");
  var chips = Array.prototype.slice.call(document.querySelectorAll(".chip"));
  var none = document.getElementById("none");
  if (cards.length && q) {
    var params = new URLSearchParams(location.search);
    var tag = params.get("tag") || "";
    q.value = params.get("q") || "";
    var apply = function () {
      var words = q.value.toLowerCase().split(/\s+/).filter(Boolean);
      var shown = 0;
      cards.forEach(function (c) {
        var ok = words.every(function (w) { return c.dataset.search.indexOf(w) >= 0; }) &&
          (!tag || (" " + c.dataset.tags + " ").indexOf(" " + tag + " ") >= 0);
        c.hidden = !ok;
        if (ok) shown++;
      });
      chips.forEach(function (ch) { ch.setAttribute("aria-pressed", String(ch.dataset.tag === tag)); });
      if (none) none.hidden = shown > 0;
      var p = new URLSearchParams();
      if (q.value) p.set("q", q.value);
      if (tag) p.set("tag", tag);
      var s = p.toString();
      history.replaceState(null, "", location.pathname + (s ? "?" + s : "") + location.hash);
    };
    q.addEventListener("input", apply);
    q.form.addEventListener("submit", function (e) { e.preventDefault(); apply(); });
    chips.forEach(function (ch) { ch.addEventListener("click", function () { tag = ch.dataset.tag; apply(); }); });
    apply();
  }
  document.querySelectorAll(".copy").forEach(function (b) {
    b.addEventListener("click", function () {
      if (!navigator.clipboard) return;
      navigator.clipboard.writeText(b.dataset.copy).then(function () {
        var t = b.textContent; b.textContent = "Copied"; setTimeout(function () { b.textContent = t; }, 1500);
      });
    });
  });
  // Calls to the API carry the session cookie and the header the server requires (API-7).
  function api(base, method, path, body) {
    return fetch(base + path, {
      method: method, credentials: "include",
      headers: { "X-Specfork": "1", "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined
    }).then(function (r) {
      return (r.status === 204 ? Promise.resolve(null) : r.json().catch(function () { return null; }))
        .then(function (j) { return { status: r.status, json: j }; });
    });
  }
  function el(tag, text, attrs) {
    var e = document.createElement(tag);
    if (text) e.textContent = text;
    Object.keys(attrs || {}).forEach(function (k) { e.setAttribute(k, attrs[k]); });
    return e;
  }
  function signIn(base) {
    return base + "/auth/github/login?return=" + encodeURIComponent(location.pathname);
  }
  var box = document.querySelector("[data-score-box]");
  if (box) {
    var base = box.dataset.api, spec = box.dataset.spec, ui = box.querySelector("[data-score-ui]");
    var path = "/scores/" + spec.replace(/^@/, "");
    var draw = function (me, mine) {
      ui.textContent = "";
      if (!me) {
        ui.appendChild(el("a", "Sign in with GitHub to score", { href: signIn(base) }));
      } else {
        for (var n = 1; n <= 5; n++) {
          (function (n) {
            var b = el("button", String(n), { type: "button", "aria-pressed": String(mine === n) });
            b.addEventListener("click", function () {
              api(base, "PUT", path, { score: n }).then(function (r) {
                if (r.status === 200) draw(me, n);
                else ui.appendChild(el("span", " " + (r.json && r.json.error === "account_too_new" ? "Your GitHub account is too new to score." : "Couldn't save the score."), {}));
              });
            });
            ui.appendChild(b);
            ui.appendChild(document.createTextNode(" "));
          })(n);
        }
        if (mine) {
          var rm = el("button", "Remove my score", { type: "button" });
          rm.addEventListener("click", function () { api(base, "DELETE", path).then(function () { draw(me, null); }); });
          ui.appendChild(rm);
        }
        var out = el("button", "Sign out", { type: "button" });
        out.addEventListener("click", function () { api(base, "POST", "/logout").then(function () { draw(null, null); }); });
        ui.appendChild(document.createTextNode(" "));
        ui.appendChild(out);
      }
      ui.hidden = false;
    };
    api(base, "GET", "/me").then(function (r) {
      if (r.status !== 200) return draw(null, null);
      api(base, "GET", path + "/mine").then(function (m) { draw(r.json, m.json ? m.json.score : null); });
    }).catch(function () {});
  }
  var claim = document.querySelector("[data-claim-box]");
  if (claim) {
    var cbase = claim.dataset.api, cui = claim.querySelector("[data-claim-ui]");
    claim.hidden = false;
    cui.textContent = "";
    cui.appendChild(el("a", "Sign in with GitHub", { href: signIn(cbase) }));
    api(cbase, "GET", "/claim/repos").then(function (r) {
      if (r.status !== 200) return;
      cui.textContent = r.json.length ? "Your repos with the specfork topic:" : "No unclaimed repo of yours has the specfork topic yet.";
      var ul = el("ul", "", { "class": "plain" });
      r.json.forEach(function (c) {
        var li = el("li", c.repo + " as " + (c.name || "?") + " ");
        if (c.claimable) {
          var b = el("button", "Claim", { type: "button" });
          b.addEventListener("click", function () {
            b.disabled = true;
            api(cbase, "POST", "/claim", { repo_id: c.repo_id }).then(function (res) {
              li.textContent = "";
              if (res.status === 200 || res.status === 201) {
                li.appendChild(el("a", "Claim pull request", { href: res.json.pr_url }));
              } else {
                li.textContent = c.repo + ": " + ((res.json && res.json.failures) || ["claim failed"]).join("; ");
              }
            });
          });
          li.appendChild(b);
        } else {
          li.appendChild(el("span", "can't claim yet: " + c.failures.join("; "), { "class": "muted" }));
        }
        ul.appendChild(li);
      });
      cui.appendChild(ul);
    }).catch(function () {});
  }
})();
