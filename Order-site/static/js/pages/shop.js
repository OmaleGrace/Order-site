(function () {
    const search = document.getElementById('shop-dish-search');
    const cards = Array.from(document.querySelectorAll('.shop-food-card'));
    const buttons = Array.from(document.querySelectorAll('.shop-category-btn'));
    const empty = document.getElementById('shop-no-results');
    const clear = document.getElementById('shop-clear-search');
    let category = 'all';

    function applyFilters() {
        const query = (search ? search.value : '').trim().toLowerCase();
        let visible = 0;
        cards.forEach(card => {
            const matchesCategory = category === 'all' || card.dataset.category === category;
            const matchesSearch = !query || card.dataset.search.toLowerCase().includes(query);
            const show = matchesCategory && matchesSearch;
            card.hidden = !show;
            if (show) visible++;
        });
        if (empty) empty.hidden = visible !== 0 || cards.length === 0;
    }

    buttons.forEach(button => button.addEventListener('click', function () {
        category = this.dataset.category || 'all';
        buttons.forEach(btn => btn.classList.toggle('active', btn === this));
        applyFilters();
    }));
    if (search) search.addEventListener('input', applyFilters);
    if (clear) clear.addEventListener('click', function () { if (search) { search.value = ''; search.focus(); } category = 'all'; buttons.forEach(btn => btn.classList.toggle('active', btn.dataset.category === 'all')); applyFilters(); });
    applyFilters();
})();
