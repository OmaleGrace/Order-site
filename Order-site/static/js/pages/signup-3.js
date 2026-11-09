(function () {
    var form = document.querySelector('form[action="/signup"]');
    var banner = document.getElementById('form-banner');
    var btn = form.querySelector('button[type="submit"]');
    function show(kind,text){banner.className='form-banner '+kind;banner.textContent=text;banner.hidden=false;}
    form.addEventListener('submit',function(e){
        e.preventDefault();
        if(btn.disabled)return;
        var label=btn.textContent;btn.disabled=true;btn.textContent='Creating account...';banner.hidden=true;
        fetch('/signup',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/x-www-form-urlencoded','Accept':'application/json'},body:new URLSearchParams(new FormData(form)).toString()})
        .then(function(r){return r.json().then(function(d){return{ok:r.ok,d:d}})})
        .then(function(res){
            if(res.ok&&res.d.ok){show('success','✅ Account created successfully! Taking you to the menu...');form.querySelectorAll('input,select').forEach(function(i){i.disabled=true});btn.textContent='Account created';setTimeout(function(){location.href=res.d.redirect||'/menu'},1200)}
            else{show('error',(res.d&&res.d.error)||'Something went wrong. Please try again.');btn.disabled=false;btn.textContent=label}
        }).catch(function(){show('error','Could not reach the server. Check your connection and try again.');btn.disabled=false;btn.textContent=label});
    });
})();
