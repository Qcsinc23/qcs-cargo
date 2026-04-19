// Auto-extracted from shipping-calculator.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    const rates = { guyana: 3.50, jamaica: 3.75, trinidad: 3.50, barbados: 4.00, suriname: 4.25 };

    function calculate() {
      const dest = document.getElementById('dest').value;
      const weight = parseFloat(document.getElementById('weight').value) || 0;
      const L = parseFloat(document.getElementById('len').value) || 0;
      const W = parseFloat(document.getElementById('wid').value) || 0;
      const H = parseFloat(document.getElementById('hgt').value) || 0;
      const val = parseFloat(document.getElementById('value').value) || 0;
      const insure = document.getElementById('insurance').checked;
      const svc = document.querySelector('input[name=service]:checked').value;

      if (!dest || weight <= 0) {
        alert('Please select a destination and enter weight.');
        return;
      }

      const dimWeight = L && W && H ? (L * W * H) / 166 : 0;
      const billable = Math.max(weight, dimWeight);
      const rate = rates[dest];
      let base = billable * rate;
      let surcharge = 0;
      let d2d = 0;

      if (svc === 'express') surcharge = base * 0.25;
      if (svc === 'door_to_door') d2d = 25;

      const insurance = insure ? val / 100 : 0;
      let total = base + surcharge + d2d + insurance;
      total = Math.max(total, 10);

      const rows = [
        ['Actual weight', weight.toFixed(1) + ' lbs'],
        dimWeight > 0 ? ['Dimensional weight', dimWeight.toFixed(1) + ' lbs'] : null,
        ['Billable weight', billable.toFixed(1) + ' lbs'],
        ['Base cost', '$' + base.toFixed(2)],
        surcharge > 0 ? ['Express surcharge (+25%)', '$' + surcharge.toFixed(2)] : null,
        d2d > 0 ? ['Door-to-door fee', '$' + d2d.toFixed(2)] : null,
        insurance > 0 ? ['Insurance', '$' + insurance.toFixed(2)] : null,
      ].filter(Boolean);

      document.getElementById('placeholder').style.display = 'none';
      document.getElementById('result').style.display = 'block';
      document.getElementById('total').textContent = '$' + total.toFixed(2);
      document.getElementById('breakdown').innerHTML = rows
        .map(([k, v]) => `<div style="display:flex; justify-content:space-between;"><span>${k}</span><strong>${v}</strong></div>`)
        .join('');
    }
  