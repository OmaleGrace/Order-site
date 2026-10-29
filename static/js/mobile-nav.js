(function(){
 function init(){
  if(window.matchMedia('(min-width: 769px)').matches || document.querySelector('.home-sidebar')) return;
  const header=document.querySelector('header');
  const nav=header&&header.querySelector('nav');
  if(!header||!nav||document.querySelector('.mobile-nav-overlay')) return;
  const toggle=document.createElement('button');
  toggle.className='mobile-nav-toggle';toggle.type='button';toggle.setAttribute('aria-label','Open navigation menu');toggle.setAttribute('aria-expanded','false');
  toggle.innerHTML='<span aria-hidden="true" style="font-size:22px;line-height:1">☰</span><span>Menu</span>';
  const brand=header.querySelector('.brand,.menu-header .brand,.vendor-brand,.admin-brand');
  if(brand) brand.insertAdjacentElement('afterend',toggle); else header.insertBefore(toggle,nav);
  const overlay=document.createElement('div');overlay.className='mobile-nav-overlay';
  overlay.innerHTML='<aside class="mobile-nav-drawer" role="dialog" aria-modal="true" aria-label="Main navigation"><div class="mobile-nav-drawer-head"><div class="mobile-nav-drawer-brand"></div><button class="mobile-nav-close" type="button" aria-label="Close navigation">×</button></div><div class="mobile-nav-content"></div></aside>';
  const drawerNav=nav.cloneNode(true);
  drawerNav.removeAttribute('style');
  overlay.querySelector('.mobile-nav-content').appendChild(drawerNav);
  const title=document.querySelector('.brand-name,.vendor-brand a,.admin-brand a,.brand a');
  overlay.querySelector('.mobile-nav-drawer-brand').innerHTML=(title?title.textContent.trim():'Navigation')+'<small>Browse the site</small>';
  document.body.appendChild(overlay);
  const close=()=>{overlay.classList.remove('is-open');document.body.classList.remove('mobile-nav-open');toggle.setAttribute('aria-expanded','false');};
  const open=()=>{overlay.classList.add('is-open');document.body.classList.add('mobile-nav-open');toggle.setAttribute('aria-expanded','true');overlay.querySelector('.mobile-nav-close').focus();};
  toggle.addEventListener('click',open);overlay.querySelector('.mobile-nav-close').addEventListener('click',close);
  overlay.addEventListener('click',e=>{if(e.target===overlay)close();});
  overlay.querySelectorAll('a').forEach(a=>a.addEventListener('click',close));
  document.addEventListener('keydown',e=>{if(e.key==='Escape'&&overlay.classList.contains('is-open')){close();toggle.focus();}});
  window.addEventListener('resize',()=>{if(window.matchMedia('(min-width: 769px)').matches)close();});
 }
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init);else init();
})();