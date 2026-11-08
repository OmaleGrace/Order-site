/* Time-of-day greeting. Put <span data-greeting>Welcome back</span> where the greeting goes.
   It uses the visitor's own phone clock, so it is right in any time zone. The text inside the
   span is what shows if JavaScript is off. */
(function () {
  'use strict';

  function greetingFor(hour) {
    if (hour >= 5 && hour < 12) return 'Good morning';
    if (hour >= 12 && hour < 17) return 'Good afternoon';
    return 'Good evening'; // 5 pm until 4:59 am
  }

  function apply() {
    var text = greetingFor(new Date().getHours());
    var nodes = document.querySelectorAll('[data-greeting]');
    for (var i = 0; i < nodes.length; i++) {
      if (nodes[i].textContent !== text) nodes[i].textContent = text;
    }
  }

  apply();
  // An installed app can stay open for hours: refresh the greeting when the person comes back to it
  document.addEventListener('visibilitychange', function () { if (!document.hidden) apply(); });
}());
