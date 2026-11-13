// Phone verification with Firebase.
// 1. Firebase texts a code to the number (after an invisible reCAPTCHA check).
// 2. The visitor types the code; Firebase confirms it and gives us an ID token.
// 3. We send that token to our server, which checks it with the Firebase Admin SDK and saves the number.
(function () {
  var box = document.getElementById("fb-verify");
  if (!box) return;

  var phone = box.dataset.phone;
  var pinId = box.dataset.pin;
  var statusEl = document.getElementById("fb-status");
  var sendBtn = document.getElementById("fb-send");
  var resendBtn = document.getElementById("fb-resend");
  var confirmBtn = document.getElementById("fb-confirm");
  var form = document.getElementById("fb-code-form");
  var codeInput = document.getElementById("fb-code");

  var confirmation = null;
  var verifier = null;
  var cooldownTimer = null;

  function say(text, isError) {
    statusEl.textContent = text || "";
    statusEl.style.color = isError ? "#b3261e" : "";
  }

  var cfg = window.FIREBASE_CONFIG;
  if (!cfg || !cfg.apiKey || cfg.apiKey.indexOf("PASTE_") === 0) {
    say("Phone verification is not set up yet. Please contact support.", true);
    sendBtn.disabled = true;
    return;
  }
  firebase.initializeApp(cfg);
  var auth = firebase.auth();
  auth.useDeviceLanguage();

  function friendlyError(err) {
    switch (err && err.code) {
      case "auth/invalid-phone-number": return "That phone number is not valid.";
      case "auth/too-many-requests": return "Too many tries. Please wait a while and try again.";
      case "auth/quota-exceeded": return "We cannot send more codes right now. Please try later.";
      case "auth/invalid-verification-code": return "That code is not correct. Check it and try again.";
      case "auth/code-expired": return "That code has expired. Tap Send again for a new one.";
      case "auth/captcha-check-failed": return "Security check failed. Refresh the page and try again.";
      case "auth/operation-not-allowed": return "Text messages to this country are switched off. Please contact support.";
      case "auth/network-request-failed": return "No connection. Check your internet and try again.";
      default: return "Something went wrong. Please try again.";
    }
  }

  function startCooldown(seconds) {
    clearInterval(cooldownTimer);
    resendBtn.disabled = true;
    var left = seconds;
    resendBtn.textContent = "Send again (" + left + "s)";
    cooldownTimer = setInterval(function () {
      left--;
      if (left <= 0) {
        clearInterval(cooldownTimer);
        resendBtn.disabled = false;
        resendBtn.textContent = "Send again";
      } else {
        resendBtn.textContent = "Send again (" + left + "s)";
      }
    }, 1000);
  }

  function sendCode(button) {
    button.disabled = true;
    say("Sending code…");
    if (verifier) { try { verifier.clear(); } catch (e) {} }
    verifier = new firebase.auth.RecaptchaVerifier("fb-recaptcha", { size: "invisible" });
    auth.signInWithPhoneNumber(phone, verifier).then(function (result) {
      confirmation = result;
      say("Code sent. It can take a minute to arrive.");
      sendBtn.hidden = true;
      form.hidden = false;
      codeInput.focus();
      startCooldown(60);
    }).catch(function (err) {
      say(friendlyError(err), true);
      button.disabled = false;
      if (verifier) { try { verifier.clear(); } catch (e) {} verifier = null; }
    });
  }

  sendBtn.addEventListener("click", function () { sendCode(sendBtn); });
  resendBtn.addEventListener("click", function () { sendCode(resendBtn); });

  form.addEventListener("submit", function (e) {
    e.preventDefault();
    if (!confirmation) return;
    confirmBtn.disabled = true;
    say("Checking code…");
    confirmation.confirm(codeInput.value.trim()).then(function (cred) {
      return cred.user.getIdToken();
    }).then(function (idToken) {
      return fetch("/account/phone/firebase-verify", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ pin_id: pinId, id_token: idToken })
      }).then(function (res) {
        return res.json().then(function (body) { return { ok: res.ok, body: body }; });
      });
    }).then(function (r) {
      if (!r.ok) throw { custom: r.body.error || "Could not verify. Please try again." };
      auth.signOut();
      window.location.href = r.body.redirect || "/account";
    }).catch(function (err) {
      say(err && err.custom ? err.custom : friendlyError(err), true);
      confirmBtn.disabled = false;
    });
  });
})();
