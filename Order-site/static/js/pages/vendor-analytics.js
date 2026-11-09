(function(){
  const bars=[...document.querySelectorAll('.bar-col')]; const values=bars.map(b=>Number(b.dataset.sales||0)); const max=Math.max(...values,1);
  bars.forEach(b=>{const v=Number(b.dataset.sales||0); b.querySelector('.bar').style.height=Math.max(4,(v/max)*170)+'px';});
})();
