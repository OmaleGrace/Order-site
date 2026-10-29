(function () {
    var form = document.getElementById('apply-form');
    if (!form) return;

    function el(id) { return document.getElementById(id); }
    function len(v) { return [...v.trim()].length; }

    var checks = {
        business_name: function (v) {
            var n = len(v);
            if (n === 0) return 'Enter your business name';
            if (n < 3) return 'The business name is too short (at least 3 characters)';
            if (n > 80) return 'The business name must be 80 characters or fewer';
            return '';
        },
        area: function (v) {
            var n = len(v);
            if (n === 0) return 'Enter the area your shop is in';
            if (n < 2) return 'The area is too short';
            return '';
        },
        address: function (v) {
            var n = len(v);
            if (n === 0) return 'Enter your shop address';
            if (n < 8) return 'Enter a fuller address (at least 8 characters)';
            return '';
        },
        online_link: function (v) {
            v = v.trim();
            if (v === '') return '';
            return /^https?:\/\/\S+\.\S+$/.test(v) ? '' : 'The link must start with https:// (or leave it empty)';
        },
        owner_name: function (v) {
            var words = v.trim().split(/\s+/).filter(Boolean);
            if (words.length < 2) return 'Enter your full legal name, first and last name';
            for (var i = 0; i < words.length; i++) {
                if ([...words[i]].length < 2) return 'Each part of your name must be at least 2 letters';
                if (!/^[\p{L}\-'’.]+$/u.test(words[i])) return 'Your name can only contain letters';
            }
            return '';
        },
        phone: function (v) {
            v = v.trim();
            if (v === '') return 'Enter your phone number';
            if (!/^\+?[0-9 \-]+$/.test(v)) return 'Use only numbers, spaces, + and -';
            var d = v.replace(/[^0-9]/g, '');
            var international = v.charAt(0) === '+' || d.indexOf('234') === 0;
            if (international) {
                if (d.indexOf('234') !== 0) return 'Start with 234 after the + sign, for example +2348012345678';
                if (d.length < 13) return 'Too short: after +234 you need 10 more digits (you have ' + (d.length - 3) + ')';
                if (d.length > 13) return 'Too long: after +234 you need exactly 10 digits (you have ' + (d.length - 3) + ')';
                d = '0' + d.slice(3);
            } else {
                if (d.charAt(0) !== '0') return 'Start with 0, for example 08012345678, or with +234';
                if (d.length < 11) return 'Too short: a Nigerian number has 11 digits (you have ' + d.length + ')';
                if (d.length > 11) return 'Too long: a Nigerian number has 11 digits (you have ' + d.length + ')';
            }
            if ('789'.indexOf(d.charAt(1)) === -1) return 'Enter a valid Nigerian mobile number, for example 08012345678';
            return '';
        },
        id_type: function (v) { return v === '' ? 'Choose the kind of ID you will show us' : ''; },
        notes: function (v) { return len(v) > 500 ? 'The notes must be 500 characters or fewer' : ''; }
    };

    var names = Object.keys(checks);
    var touched = {};
    var submitBtn = el('submit-btn');
    var confirmBox = el('confirm');

    function refresh() {
        var allOk = confirmBox.checked;
        names.forEach(function (name) {
            var input = el(name);
            var err = checks[name](input.value);
            if (err !== '') allOk = false;
            var visible = !!touched[name];
            el(name + '-error').textContent = visible ? err : '';
            input.classList.toggle('invalid', visible && err !== '');
        });
        submitBtn.disabled = !allOk;
    }

    names.forEach(function (name) {
        ['input', 'change', 'blur'].forEach(function (ev) {
            el(name).addEventListener(ev, function () { touched[name] = true; refresh(); });
        });
    });
    confirmBox.addEventListener('change', refresh);

    form.addEventListener('submit', function (e) {
        names.forEach(function (name) { touched[name] = true; });
        refresh();
        if (submitBtn.disabled) { e.preventDefault(); return; }
        submitBtn.disabled = true;
        submitBtn.textContent = 'Sending...';
    });

    window.addEventListener('pageshow', function () {
        submitBtn.textContent = 'Send my application';
        refresh();
    });

    refresh();
})();
