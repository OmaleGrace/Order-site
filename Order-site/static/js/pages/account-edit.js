(function () {
  "use strict";
  var input = document.getElementById("profile_picture");
  if (!input) return;

  var previewIds = ["profile-preview", "profile-preview-large"];
  var placeholderIds = ["profile-placeholder", "profile-placeholder-large"];
  var fileName = document.getElementById("profile-file-name");
  var activeURL = null;

  input.addEventListener("change", function () {
    var file = input.files && input.files[0];
    if (!file) {
      if (fileName) fileName.textContent = "No new photo selected";
      return;
    }
    if (!/^image\/(jpeg|png|webp)$/.test(file.type)) {
      input.value = "";
      if (fileName) fileName.textContent = "Choose a JPG, PNG or WEBP image.";
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      input.value = "";
      if (fileName) fileName.textContent = "That image is larger than 5MB. Please choose a smaller one.";
      return;
    }

    if (activeURL) URL.revokeObjectURL(activeURL);
    activeURL = URL.createObjectURL(file);

    previewIds.forEach(function (id) {
      var img = document.getElementById(id);
      if (img) {
        img.src = activeURL;
        img.hidden = false;
      }
    });
    placeholderIds.forEach(function (id) {
      var placeholder = document.getElementById(id);
      if (placeholder) placeholder.hidden = true;
    });
    if (fileName) fileName.textContent = file.name;
  });

  window.addEventListener("beforeunload", function () {
    if (activeURL) URL.revokeObjectURL(activeURL);
  });
})();
