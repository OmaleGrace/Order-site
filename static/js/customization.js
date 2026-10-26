(function () {
  'use strict';

  function formatNaira(kobo) {
    return '₦' + (Number(kobo || 0) / 100).toLocaleString('en-NG', {minimumFractionDigits: 0, maximumFractionDigits: 2});
  }

  function setupCustomization(modal) {
    if (!modal) return;
    const form = modal.querySelector('[data-custom-form]');
    const groupsEl = modal.querySelector('[data-custom-groups]');
    const titleEl = modal.querySelector('[data-custom-title]');
    const baseEl = modal.querySelector('[data-custom-base]');
    const totalEl = modal.querySelector('[data-custom-total]');
    const itemEl = modal.querySelector('[data-custom-item-id]');
    const errorEl = modal.querySelector('[data-custom-error]');
    const closeBtn = modal.querySelector('[data-custom-close]');
    if (!form || !groupsEl || !itemEl) return;

    let basePrice = 0;

    function close() {
      modal.hidden = true;
      groupsEl.innerHTML = '';
      if (errorEl) errorEl.hidden = true;
      form.reset();
    }

    function selectedPrice() {
      return [...form.querySelectorAll('input[name="option_id"]:checked')]
        .reduce((sum, input) => sum + Number(input.dataset.price || 0), 0);
    }

    function updateTotal() {
      if (baseEl) baseEl.textContent = formatNaira(basePrice);
      if (totalEl) totalEl.textContent = formatNaira(basePrice + selectedPrice());
    }

    function showError(message) {
      if (!errorEl) return;
      errorEl.textContent = message;
      errorEl.hidden = false;
    }

    function validate(showMessage) {
      let valid = true;
      let firstError = '';
      groupsEl.querySelectorAll('[data-group]').forEach(group => {
        const inputs = [...group.querySelectorAll('input[name="option_id"]')];
        if (!inputs.length) return;
        const min = Number(inputs[0].dataset.min || 0);
        const max = Number(inputs[0].dataset.max || inputs.length);
        const count = inputs.filter(i => i.checked).length;
        const fieldset = group;
        const bad = count < min || count > max;
        fieldset.classList.toggle('custom-group-error', bad);
        if (bad && valid) {
          valid = false;
          firstError = min === max
            ? `Please choose ${min} option${min === 1 ? '' : 's'} from ${inputs[0].dataset.groupName}.`
            : `Please choose between ${min} and ${max} options from ${inputs[0].dataset.groupName}.`;
        }
      });
      if (!valid && showMessage) showError(firstError || 'Please check your selections.');
      if (valid && errorEl) errorEl.hidden = true;
      return valid;
    }

    function renderGroups(data) {
      groupsEl.innerHTML = '';
      data.forEach(group => {
        const fieldset = document.createElement('fieldset');
        fieldset.className = 'custom-group';
        fieldset.dataset.group = String(group.id);

        const legend = document.createElement('legend');
        const title = document.createElement('span');
        title.className = 'custom-group-name';
        title.textContent = group.name;
        const rule = document.createElement('span');
        rule.className = 'custom-group-rule';
        rule.textContent = group.min_select === group.max_select
          ? `Choose ${group.min_select}`
          : `Choose ${group.min_select}–${group.max_select}`;
        legend.append(title, rule);
        fieldset.appendChild(legend);

        const list = document.createElement('div');
        list.className = 'custom-option-list';
        group.options.forEach(option => {
          const label = document.createElement('label');
          label.className = 'custom-option';
          const input = document.createElement('input');
          input.type = 'checkbox';
          input.name = 'option_id';
          input.value = option.id;
          input.dataset.group = group.id;
          input.dataset.groupName = group.name;
          input.dataset.min = group.min_select;
          input.dataset.max = group.max_select;
          input.dataset.price = option.price_kobo;
          input.addEventListener('change', function () {
            const checked = [...fieldset.querySelectorAll('input[name="option_id"]:checked')];
            if (checked.length > group.max_select) {
              this.checked = false;
              showError(`You can choose up to ${group.max_select} option${group.max_select === 1 ? '' : 's'} from ${group.name}.`);
            }
            validate(false);
            updateTotal();
          });
          const text = document.createElement('span');
          text.className = 'custom-option-copy';
          const name = document.createElement('strong');
          name.textContent = option.name;
          const price = document.createElement('span');
          price.textContent = option.price_kobo ? `+ ${formatNaira(option.price_kobo)}` : 'Included';
          text.append(name, price);
          label.append(input, text);
          list.appendChild(label);
        });
        fieldset.appendChild(list);
        groupsEl.appendChild(fieldset);
      });
      updateTotal();
    }

    closeBtn && closeBtn.addEventListener('click', close);
    modal.addEventListener('click', e => { if (e.target === modal) close(); });

    document.querySelectorAll('[data-customize-item]').forEach(trigger => {
      trigger.addEventListener('click', async function (e) {
        e.preventDefault();
        const itemID = this.dataset.customizeItem;
        const itemName = this.dataset.itemName || 'Customize your dish';
        basePrice = Number(this.dataset.basePrice || 0);
        itemEl.value = itemID;
        if (titleEl) titleEl.textContent = itemName;
        groupsEl.innerHTML = '<div class="custom-loading">Loading available choices…</div>';
        modal.hidden = false;
        try {
          const response = await fetch('/menu/options?id=' + encodeURIComponent(itemID), {headers: {'Accept': 'application/json'}});
          if (!response.ok) throw new Error('options');
          const data = await response.json();
          if (!Array.isArray(data) || !data.length) {
            form.submit();
            return;
          }
          renderGroups(data);
        } catch (_) {
          groupsEl.innerHTML = '';
          showError('Could not load customization choices. Please try again.');
        }
      });
    });

    form.addEventListener('submit', function (e) {
      if (!validate(true)) e.preventDefault();
    });
  }

  document.querySelectorAll('[data-customization-modal]').forEach(setupCustomization);
})();
