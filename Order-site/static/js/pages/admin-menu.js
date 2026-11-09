(function () {
    var form = document.getElementById('item-form');
    var title = document.getElementById('form-title');
    var saveBtn = document.getElementById('save-btn');
    var cancelBtn = document.getElementById('cancel-btn');
    var f = {
        id: document.getElementById('item-id'),
        name: document.getElementById('name'),
        price: document.getElementById('price'),
        category: document.getElementById('category'),
        image: document.getElementById('image_url'),
        description: document.getElementById('description')
    };

    function resetForm() {
        form.reset();
        f.id.value = '';
        title.textContent = 'Add a menu item';
        saveBtn.textContent = 'Add item';
        cancelBtn.style.display = 'none';
    }

    document.querySelectorAll('.edit-btn').forEach(function (btn) {
        btn.addEventListener('click', function () {
            f.id.value = btn.dataset.id;
            f.name.value = btn.dataset.name;
            f.price.value = btn.dataset.price;
            f.category.value = btn.dataset.category;
            f.image.value = btn.dataset.image;
            f.description.value = btn.dataset.description;
            title.textContent = 'Edit menu item';
            saveBtn.textContent = 'Save changes';
            cancelBtn.style.display = 'inline-block';
            window.scrollTo({ top: 0, behavior: 'smooth' });
            f.name.focus();
        });
    });

    cancelBtn.addEventListener('click', resetForm);
})();
document.getElementById('select-all')?.addEventListener('change', function(){document.querySelectorAll('.item-check').forEach(function(c){c.checked=this.checked},this)});
    document.getElementById('menu-filter')?.addEventListener('input', function(){var q=this.value.toLowerCase().trim();document.querySelectorAll('.menu-row').forEach(function(row){row.style.display=!q||row.dataset.search.toLowerCase().includes(q)?'':'none'})});
