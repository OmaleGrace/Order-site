(function(){
  var tool=document.getElementById('tool');
  var dish=document.getElementById('dish_name');
  var message=document.getElementById('customer_message');
  var dishField=document.getElementById('dish-field');
  var messageField=message ? message.closest('.ai-field') : null;
  var toolButtons=document.querySelectorAll('.ai-tool');

  function syncAIFields(){
    if(!tool) return;
    var chat=tool.value==='customer_chat';
    var brief=tool.value==='business_brief';
    if(dish) dish.required=!chat && !brief;
    if(dish) dish.disabled=brief;
    if(message) message.disabled=brief;
    if(message) message.required=chat;
    if(dishField) dishField.classList.toggle('chat-mode',chat);
    if(messageField) messageField.classList.toggle('chat-mode',!chat);
    toolButtons.forEach(function(btn){
      btn.classList.toggle('active',btn.getAttribute('data-tool')===tool.value);
    });
  }

  if(tool){
    tool.addEventListener('change',syncAIFields);
    toolButtons.forEach(function(btn){
      btn.addEventListener('click',function(){
        tool.value=btn.getAttribute('data-tool');
        tool.dispatchEvent(new Event('change'));
        if(tool.value==='customer_chat' && message) message.focus();
        else if(dish) dish.focus();
      });
    });
    syncAIFields();
  }

  var copy=document.getElementById('copy-result');
  if(copy){
    copy.addEventListener('click',function(){
      var output=document.getElementById('ai-result');
      var status=document.getElementById('copy-status');
      if(!output) return;
      navigator.clipboard.writeText(output.innerText).then(function(){
        status.textContent='Copied';
        setTimeout(function(){status.textContent=''},1800);
      }).catch(function(){
        status.textContent='Select the draft and copy manually.';
      });
    });
  }
})();
