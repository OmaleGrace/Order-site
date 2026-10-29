(() => {
    const liveOrders = document.querySelectorAll('.order-live');
    if (liveOrders.length) {
        setTimeout(() => window.location.reload(), 15000);
    }
})();
