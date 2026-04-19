// Auto-extracted from recipients.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent('/dashboard/recipients');
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      var state = { list: [], editingId: null };

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/recipients').catch(function () { return { data: [] }; })
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        state.list = (results[1] && results[1].data) || [];
        renderShell();
        QCS.bindLogout();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load recipients',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function val(r, key) {
        var v = r[key];
        if (v && typeof v === 'object' && 'String' in v) return v.Valid ? v.String : '';
        return (typeof v === 'string' || typeof v === 'number') ? String(v) : '';
      }

      function renderShell() {
        var sidebar = QCS.renderSidebar('recipients');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Recipients</li></ol></nav>'
          + '<h1 class="text-3xl font-bold text-[#0F172A] mb-2">Recipients</h1>'
          + '<p class="text-slate-600 mb-6">Saved shipping addresses for your packages.</p>'
          + '<div id="recipient-form-section">' + renderForm() + '</div>'
          + '<div id="recipient-list-section" class="mt-6">' + renderList() + '</div>'
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });
        bindForm();
        bindList();
      }

      function renderForm() {
        var rec = state.editingId ? state.list.filter(function (r) { return r.id === state.editingId; })[0] : null;
        var isEditing = !!rec;
        return '<section class="bg-white rounded-xl border border-slate-200 shadow-sm p-6">'
          + '<h2 class="text-lg font-semibold text-[#0F172A] mb-4">' + (isEditing ? 'Edit recipient' : 'Add recipient') + '</h2>'
          + '<form id="recipient-form" class="grid grid-cols-1 md:grid-cols-2 gap-4">'
          + '<input type="hidden" id="form-id" value="' + window.qcsEscapeHTML(state.editingId || '') + '">'
          + textField('form-name', 'Name', 'text', rec ? val(rec, 'name') : '', 'Full name', true)
          + textField('form-phone', 'Phone', 'tel', rec ? val(rec, 'phone') : '', 'Phone')
          + textField('form-destination_id', 'Destination', 'text', rec ? val(rec, 'destination_id') : '', 'e.g. guyana', true)
          + textField('form-street', 'Street', 'text', rec ? val(rec, 'street') : '', 'Street address', true)
          + textField('form-apt', 'Apt / Unit', 'text', rec ? val(rec, 'apt') : '', 'Apt, suite')
          + textField('form-city', 'City', 'text', rec ? val(rec, 'city') : '', 'City', true)
          + '<div class="md:col-span-2">'
          + '<label for="form-delivery_instructions" class="block text-sm font-medium text-slate-700 mb-1">Delivery instructions</label>'
          + '<textarea id="form-delivery_instructions" rows="2" class="w-full border border-slate-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-[#2563EB]" placeholder="Optional instructions">' + window.qcsEscapeHTML(rec ? val(rec, 'delivery_instructions') : '') + '</textarea>'
          + '</div>'
          + '<label class="flex items-center gap-2 text-sm text-slate-700">'
          + '<input type="checkbox" id="form-is_default" class="h-4 w-4 rounded border-slate-300 text-[#2563EB] focus:ring-[#2563EB]"' + (rec && rec.is_default ? ' checked' : '') + '>'
          + 'Default recipient'
          + '</label>'
          + '<div class="md:col-span-2 flex flex-wrap items-center gap-3 pt-3 border-t border-slate-100">'
          + '<button type="submit" class="bg-[#F97316] text-white px-6 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">' + (isEditing ? 'Update recipient' : 'Add recipient') + '</button>'
          + (isEditing ? '<button type="button" id="form-cancel" class="bg-white border border-slate-300 px-4 py-2 rounded-lg font-medium hover:bg-slate-50">Cancel</button>' : '')
          + '</div>'
          + '</form></section>';
      }

      function textField(id, label, type, value, placeholder, required) {
        return '<div>'
          + '<label for="' + id + '" class="block text-sm font-medium text-slate-700 mb-1">' + window.qcsEscapeHTML(label) + (required ? ' *' : '') + '</label>'
          + '<input type="' + (type || 'text') + '" id="' + id + '" value="' + window.qcsEscapeHTML(value || '') + '"' + (required ? ' required' : '') + ' class="w-full border border-slate-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-[#2563EB]" placeholder="' + window.qcsEscapeHTML(placeholder || '') + '">'
          + '</div>';
      }

      function renderList() {
        if (!state.list.length) {
          return QCS.renderEmptyState({
            title: 'No recipients yet',
            description: 'Add a recipient above so you can ship packages to them.',
            actionHref: '/dashboard/mailbox',
            actionLabel: 'Open mailbox'
          });
        }
        var html = '<section class="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">'
          + '<div class="overflow-x-auto"><table class="w-full">'
          + '<thead class="bg-slate-50 border-b border-slate-200"><tr>'
          + th('Name') + th('Destination') + th('Address') + th('Default') + '<th class="py-3 px-4"></th>'
          + '</tr></thead><tbody>';
        state.list.forEach(function (r) {
          var name = val(r, 'name');
          var dest = val(r, 'destination_id');
          var street = val(r, 'street');
          var apt = val(r, 'apt');
          var city = val(r, 'city');
          var addr = street + (apt ? ', ' + apt : '') + (city ? ', ' + city : '');
          html += '<tr class="border-b border-slate-100 hover:bg-slate-50">'
            + '<td class="py-3 px-4 font-medium">' + window.qcsEscapeHTML(name) + '</td>'
            + '<td class="py-3 px-4">' + window.qcsEscapeHTML(dest) + '</td>'
            + '<td class="py-3 px-4 text-slate-600">' + window.qcsEscapeHTML(addr) + '</td>'
            + '<td class="py-3 px-4">' + (r.is_default ? '<span class="qcs-badge qcs-badge-info">Default</span>' : '<span class="text-slate-400 text-sm">—</span>') + '</td>'
            + '<td class="py-3 px-4 text-right whitespace-nowrap">'
            + '<button type="button" class="edit-btn text-[#2563EB] hover:underline font-medium mr-3" data-id="' + window.qcsEscapeHTML(r.id) + '">Edit</button>'
            + '<button type="button" class="delete-btn text-red-600 hover:underline font-medium" data-id="' + window.qcsEscapeHTML(r.id) + '">Delete</button>'
            + '</td></tr>';
        });
        html += '</tbody></table></div></section>';
        return html;
      }

      function th(label) {
        return '<th class="text-left py-3 px-4 font-medium text-slate-600 text-sm whitespace-nowrap">' + window.qcsEscapeHTML(label) + '</th>';
      }

      function bindForm() {
        var form = document.getElementById('recipient-form');
        if (!form) return;
        form.addEventListener('submit', function (e) {
          e.preventDefault();
          var id = document.getElementById('form-id').value;
          var payload = {
            name: document.getElementById('form-name').value.trim(),
            phone: document.getElementById('form-phone').value.trim(),
            destination_id: document.getElementById('form-destination_id').value.trim(),
            street: document.getElementById('form-street').value.trim(),
            apt: document.getElementById('form-apt').value.trim(),
            city: document.getElementById('form-city').value.trim(),
            delivery_instructions: document.getElementById('form-delivery_instructions').value.trim(),
            is_default: document.getElementById('form-is_default').checked ? 1 : 0
          };
          if (!payload.name || !payload.destination_id || !payload.street || !payload.city) {
            QCS.toast('Name, destination, street, and city are required', 'warning');
            return;
          }
          var req = id
            ? QCS.fetchJson('/api/v1/recipients/' + encodeURIComponent(id), { method: 'PATCH', body: payload })
            : QCS.fetchJson('/api/v1/recipients', { method: 'POST', body: payload });
          req.then(function () {
            QCS.toast(id ? 'Recipient updated' : 'Recipient added', 'success');
            state.editingId = null;
            return QCS.fetchJson('/api/v1/recipients');
          }).then(function (j) {
            state.list = (j && j.data) || [];
            refresh();
          }).catch(function (err) {
            QCS.toast((err && err.message) || 'Save failed', 'error');
          });
        });

        var cancel = document.getElementById('form-cancel');
        if (cancel) cancel.addEventListener('click', function () { state.editingId = null; refresh(); });
      }

      function bindList() {
        document.querySelectorAll('.edit-btn').forEach(function (btn) {
          btn.addEventListener('click', function () {
            state.editingId = btn.getAttribute('data-id');
            refresh();
            window.scrollTo({ top: 0, behavior: 'smooth' });
          });
        });
        document.querySelectorAll('.delete-btn').forEach(function (btn) {
          btn.addEventListener('click', function () {
            if (!confirm('Delete this recipient?')) return;
            var id = btn.getAttribute('data-id');
            QCS.fetchJson('/api/v1/recipients/' + encodeURIComponent(id), { method: 'DELETE' })
              .then(function () { return QCS.fetchJson('/api/v1/recipients'); })
              .then(function (j) {
                state.list = (j && j.data) || [];
                refresh();
                QCS.toast('Recipient deleted', 'success');
              })
              .catch(function (err) { QCS.toast((err && err.message) || 'Delete failed', 'error'); });
          });
        });
      }

      function refresh() {
        var formSection = document.getElementById('recipient-form-section');
        var listSection = document.getElementById('recipient-list-section');
        if (formSection) formSection.innerHTML = renderForm();
        if (listSection) listSection.innerHTML = renderList();
        bindForm();
        bindList();
      }
    })();
  