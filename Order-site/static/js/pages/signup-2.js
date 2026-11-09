(function () {
    var password = document.getElementById('password');
    var bar = document.getElementById('password-strength-bar');
    var label = document.getElementById('password-strength-label');
    function strength(v){
        var score=0;
        if(v.length>=8) score++;
        if(/[a-z]/.test(v)) score++;
        if(/[A-Z]/.test(v)) score++;
        if(/[0-9]/.test(v)) score++;
        if(/[^A-Za-z0-9]/.test(v)) score++;
        return score;
    }
    function update(){
        var score=strength(password.value);
        var labels=['Password strength','Very weak','Weak','Fair','Good','Strong'];
        bar.style.width=(score*20)+'%';
        label.textContent=labels[score];
        label.dataset.score=String(score);
    }
    password.addEventListener('input',update);
    update();
})();
