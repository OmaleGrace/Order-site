document.querySelectorAll('.toggle-password').forEach(function (btn) {
    btn.addEventListener('click', function () {
        var input = document.getElementById(btn.dataset.target);
        var isHidden = input.type === 'password';
        input.type = isHidden ? 'text' : 'password';
        btn.querySelector('.eye-open').style.display = isHidden ? 'none' : 'block';
        btn.querySelector('.eye-closed').style.display = isHidden ? 'block' : 'none';
        btn.setAttribute('aria-label', isHidden ? 'Hide password' : 'Show password');
    });
});

(function () {
    var first = document.getElementById('first_name');
    var middle = document.getElementById('middle_name');
    var last = document.getElementById('last_name');
    var email = document.getElementById('email');
    var phone = document.getElementById('phone');
    var country = document.getElementById('country');
    var state = document.getElementById('state');
    var pass = document.getElementById('password');
    var confirm = document.getElementById('confirm_password');
    var terms = document.getElementById('terms_accepted');
    var termsBox = document.getElementById('terms-box');
    var submitBtn = document.querySelector('form button[type="submit"]');

    function checkName(v, label, required) {
        v = v.trim();
        if (!v) return required ? label + ' is required' : '';
        if ([...v].length < 2) return label + ' must be at least 2 letters';
        return /^[\p{L}\s'\-]+$/u.test(v) ? '' : label + ' can only contain letters, spaces, hyphens or apostrophes';
    }
    function checkEmail(v) { return /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/.test(v.trim()) ? '' : 'Enter a valid email address'; }
    function checkPhone(v) { return /^(?:0\d{10}|\+234\d{10}|234\d{10})$/.test(v.trim().replace(/[\s-]/g, '')) ? '' : 'Enter a valid Nigerian phone number'; }
    function checkPassword(v) {
        if ([...v].length < 8) return 'Password must be at least 8 characters';
        if (!/\p{Lu}/u.test(v)) return 'Add a capital letter';
        if (!/\p{Ll}/u.test(v)) return 'Add a lowercase letter';
        if (!/\p{Nd}/u.test(v)) return 'Add a number';
        if (!/[^\p{L}\p{N}\s]/u.test(v)) return 'Add a special character, like ! @ # $';
        return '';
    }

    var fields = [
        {el:first, check:function(v){return checkName(v,'First name',true)}, msg:document.getElementById('first-name-error')},
        {el:middle, check:function(v){return checkName(v,'Middle name',false)}, msg:document.getElementById('middle-name-error')},
        {el:last, check:function(v){return checkName(v,'Last name',true)}, msg:document.getElementById('last-name-error')},
        {el:email, check:checkEmail, msg:document.getElementById('email-error')},
        {el:phone, check:checkPhone, msg:document.getElementById('phone-error')},
        {el:country, check:function(v){return v==='Nigeria'?'':'Select Nigeria as your country'}, msg:document.getElementById('country-error')},
        {el:state, check:function(v){return v?'':'Select your state'}, msg:document.getElementById('state-error')},
        {el:pass, check:checkPassword, msg:document.getElementById('password-error')},
        {el:confirm, check:function(v){return v===pass.value&&v!==''?'':'Passwords do not match'}, msg:document.getElementById('confirm-password-error')}
    ];
    function update(f) { var err=f.check(f.el.value); f.msg.textContent=err; if(f.el.tagName==='INPUT') f.el.classList.toggle('invalid',!!err); return err; }
    function refresh() {
        var valid=fields.every(function(f){return f.check(f.el.value)==='';}) && terms.checked;
        submitBtn.disabled=!valid;
        termsBox.classList.toggle('invalid', !terms.checked && termsBox.dataset.touched==='1');
        document.getElementById('terms-error').textContent = (!terms.checked && termsBox.dataset.touched==='1') ? 'You must accept the terms before creating your account' : '';
    }
    fields.forEach(function(f){f.el.addEventListener('input',function(){update(f);refresh()});f.el.addEventListener('change',function(){update(f);refresh()});f.el.addEventListener('blur',function(){if(f.el.value)update(f);refresh()})});
    terms.addEventListener('change',function(){termsBox.dataset.touched='1';refresh()});
    termsBox.addEventListener('click',function(){termsBox.dataset.touched='1';setTimeout(refresh,0)});
    submitBtn.disabled=true;
})();
