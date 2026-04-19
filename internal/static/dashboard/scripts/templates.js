// Auto-extracted from templates.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so
// the CSP can drop 'unsafe-inline' (Phase 3.1).

    (function () {
      'use strict';
      var QCS = window.QCSPWA;
      QCS.initBase({ registerSW: true, keyboard: true, utilityDock: true });

      var loginRedirect = '/login?redirect=' + encodeURIComponent('/dashboard/templates');
      var token = QCS.readToken();
      if (!token) { window.location.href = loginRedirect; return; }

      var app = document.getElementById('dashboard-app');
      QCS.mountLoading(app, QCS.t('loading'));

      var state = { list: [], destinations: [], recipients: [] };

      Promise.all([
        QCS.fetchJson('/api/v1/me'),
        QCS.fetchJson('/api/v1/templates').catch(function () { return { data: [] }; }),
        QCS.fetchJson('/api/v1/destinations').catch(function () { return { data: [] }; }),
        QCS.fetchJson('/api/v1/recipients').catch(function () { return { data: [] }; })
      ]).then(function (results) {
        var me = results[0];
        if (!me || !me.data) { window.location.href = loginRedirect; return; }
        state.list = (results[1] && results[1].data) || [];
        state.destinations = (results[2] && results[2].data) || [];
        state.recipients = (results[3] && results[3].data) || [];
        renderShell();
        QCS.bindLogout();
      }).catch(function (err) {
        QCS.mountError(app, {
          title: 'Unable to load templates',
          description: (err && err.message) || 'Network error.',
          actionLabel: 'Sign in again',
          onRetry: function () { window.location.href = loginRedirect; }
        });
      });

      function destName(id) {
        if (!id) return '—';
        var d = state.destinations.filter(function (x) { return (x.id || x.ID) === id; })[0];
        return d ? (d.name || d.Name || id) : id;
      }
      function recipName(id) {
        if (!id) return '—';
        var r = state.recipients.filter(function (x) { return (x.id || x.ID) === id; })[0];
        return r ? (typeof r.name === 'string' ? r.name : (r.name && r.name.String) || '—') : id;
      }
      function destOptions(selectedId) {
        var s = '<option value="">Select destination</option>';
        state.destinations.forEach(function (d) {
          var id = d.id || d.ID;
          var name = d.name || d.Name || id;
          s += '<option value="' + window.qcsEscapeHTML(id) + '"' + (id === selectedId ? ' selected' : '') + '>' + window.qcsEscapeHTML(name) + '</option>';
        });
        return s;
      }
      function recipOptions(selectedId) {
        var s = '<option value="">None</option>';
        state.recipients.forEach(function (r) {
          var id = r.id || r.ID;
          var name = typeof r.name === 'string' ? r.name : (r.name && r.name.String) || 'Unnamed';
          s += '<option value="' + window.qcsEscapeHTML(id) + '"' + (id === selectedId ? ' selected' : '') + '>' + window.qcsEscapeHTML(name) + '</option>';
        });
        return s;
      }

      function renderShell() {
        var sidebar = QCS.renderSidebar('templates');
        var html = '<div class="qcs-page-wrap">' + sidebar
          + '<main id="qcs-main" class="qcs-page-main" tabindex="-1">'
          + '<nav aria-label="Breadcrumb" class="mb-4"><ol class="flex items-center gap-2 text-sm text-slate-600">'
          + '<li><a href="/dashboard" class="text-[#2563EB] hover:underline">Dashboard</a></li>'
          + '<li aria-hidden="true">/</li>'
          + '<li class="text-[#0F172A] font-medium" aria-current="page">Templates</li></ol></nav>'
          + '<h1 class="text-3xl font-bold text-[#0F172A] mb-2">Booking Templates</h1>'
          + '<p class="text-slate-600 mb-6">Save common ship-request configurations to reuse later. ' + state.list.length + ' template(s).</p>'
          + renderCreate()
          + '<div id="templates-list" class="mt-6">' + renderTable() + '</div>'
          + '</main></div>';
        app.innerHTML = html;
        var main = document.getElementById('qcs-main');
        if (main) main.focus({ preventScroll: true });
        bindCreate();
        bindRows();
      }

      function renderCreate() {
        return '<section class="bg-white rounded-xl border border-slate-200 shadow-sm p-6">'
          + '<h2 class="text-lg font-semibold text-[#0F172A] mb-4">Create template</h2>'
          + '<form id="create-template-form" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 items-end">'
          + textField('Name', 'name', 'text', '', 'e.g. Home shipping', true)
          + selectField('Service type', 'service_type', '<option value="standard">Standard</option><option value="express">Express</option>')
          + selectField('Destination', 'destination_id', destOptions(''), true)
          + selectField('Recipient', 'recipient_id', recipOptions(''))
          + '<div class="md:col-span-2 lg:col-span-4 flex flex-wrap items-center gap-3 pt-3 border-t border-slate-100">'
          + '<button type="submit" class="bg-[#2563EB] text-white px-6 py-2 rounded-lg font-semibold shadow-sm hover:opacity-95">Save as template</button>'
          + '<p id="create-msg" class="text-sm hidden" aria-live="polite"></p>'
          + '</div>'
          + '</form></section>';
      }

      function textField(label, name, type, value, placeholder, required) {
        return '<div>'
          + '<label class="block text-sm font-medium text-slate-700 mb-1">' + window.qcsEscapeHTML(label) + (required ? ' *' : '') + '</label>'
          + '<input type="' + (type || 'text') + '" name="' + name + '" value="' + window.qcsEscapeHTML(value || '') + '"' + (required ? ' required' : '') + ' class="w-full border border-slate-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-[#2563EB]" placeholder="' + window.qcsEscapeHTML(placeholder || '') + '">'
          + '</div>';
      }

      function selectField(label, name, opts, required) {
        return '<div>'
          + '<label class="block text-sm font-medium text-slate-700 mb-1">' + window.qcsEscapeHTML(label) + (required ? ' *' : '') + '</label>'
          + '<select name="' + name + '"' + (required ? ' required' : '') + ' class="w-full border border-slate-300 rounded-lg px-3 py-2 bg-white focus:outline-none focus:ring-2 focus:ring-[#2563EB]">' + opts + '</select>'
          + '</div>';
      }

      function renderTable() {
        if (!state.list.length) {
          return QCS.renderEmptyState({
            title: 'No templates yet',
            description: 'Create one above or save a ship request as a template to speed up future shipments.',
            actionHref: '/dashboard/ship',
            actionLabel: 'Start a ship request'
          });
        }
        var html = '<section class="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">'
          + '<div class="overflow-x-auto"><table class="w-full">'
          + '<thead class="bg-slate-50 border-b border-slate-200"><tr>'
          + th('Name') + th('Service') + th('Destination') + th('Recipient') + th('Used') + '<th class="py-3 px-4"></th>'
          + '</tr></thead><tbody>';
        state.list.forEach(function (t) {
          var recipId = (t.recipient_id && (typeof t.recipient_id === 'string' ? t.recipient_id : t.recipient_id.String)) || '';
          html += '<tr class="border-b border-slate-100 hover:bg-slate-50" data-id="' + window.qcsEscapeHTML(t.id) + '">'
            + '<td class="py-3 px-4 font-medium">' + window.qcsEscapeHTML(t.name || '—') + '</td>'
            + '<td class="py-3 px-4">' + window.qcsEscapeHTML(t.service_type || '—') + '</td>'
            + '<td class="py-3 px-4">' + window.qcsEscapeHTML(destName(t.destination_id)) + '</td>'
            + '<td class="py-3 px-4">' + window.qcsEscapeHTML(recipName(recipId)) + '</td>'
            + '<td class="py-3 px-4 text-slate-500">' + (t.use_count != null ? Number(t.use_count) : 0) + '</td>'
            + '<td class="py-3 px-4 text-right whitespace-nowrap">'
            + '<button type="button" class="edit-btn text-[#2563EB] hover:underline font-medium mr-3" data-id="' + window.qcsEscapeHTML(t.id) + '">Edit</button>'
            + '<button type="button" class="del-btn text-red-600 hover:underline font-medium" data-id="' + window.qcsEscapeHTML(t.id) + '">Delete</button>'
            + '</td></tr>';
        });
        html += '</tbody></table></div></section>';
        return html;
      }

      function th(label) {
        return '<th class="text-left py-3 px-4 font-medium text-slate-600 text-sm whitespace-nowrap">' + window.qcsEscapeHTML(label) + '</th>';
      }

      function bindCreate() {
        var form = document.getElementById('create-template-form');
        if (!form) return;
        form.addEventListener('submit', function (e) {
          e.preventDefault();
          var fd = new FormData(form);
          var payload = {
            name: fd.get('name'),
            service_type: fd.get('service_type') || 'standard',
            destination_id: fd.get('destination_id'),
            recipient_id: fd.get('recipient_id') || null
          };
          if (!payload.recipient_id) delete payload.recipient_id;
          var msg = document.getElementById('create-msg');
          if (msg) { msg.textContent = 'Saving…'; msg.className = 'text-sm text-slate-500'; msg.classList.remove('hidden'); }
          QCS.fetchJson('/api/v1/templates', { method: 'POST', body: payload })
            .then(function () { return QCS.fetchJson('/api/v1/templates'); })
            .then(function (j) {
              state.list = (j && j.data) || [];
              renderShell();
              QCS.toast('Template saved', 'success');
            })
            .catch(function (err) {
              if (msg) {
                msg.textContent = (err && err.message) || 'Failed to save template.';
                msg.className = 'text-sm text-red-600';
              }
            });
        });
      }

      function bindRows() {
        document.querySelectorAll('.del-btn').forEach(function (btn) {
          btn.addEventListener('click', function () {
            if (!confirm('Delete this template? This cannot be undone.')) return;
            var id = btn.getAttribute('data-id');
            btn.disabled = true;
            QCS.fetchJson('/api/v1/templates/' + encodeURIComponent(id), { method: 'DELETE' })
              .then(function () { return QCS.fetchJson('/api/v1/templates'); })
              .then(function (j) {
                state.list = (j && j.data) || [];
                renderShell();
                QCS.toast('Template deleted', 'success');
              })
              .catch(function (err) {
                btn.disabled = false;
                QCS.toast((err && err.message) || 'Delete failed', 'error');
              });
          });
        });

        document.querySelectorAll('.edit-btn').forEach(function (btn) {
          btn.addEventListener('click', function () {
            var id = btn.getAttribute('data-id');
            var t = state.list.filter(function (x) { return x.id === id; })[0];
            if (!t) return;
            var row = document.querySelector('tr[data-id="' + id + '"]');
            if (!row || row.nextElementSibling && row.nextElementSibling.classList.contains('edit-row')) return;

            var recipId = (t.recipient_id && (typeof t.recipient_id === 'string' ? t.recipient_id : t.recipient_id.String)) || '';
            var tr = document.createElement('tr');
            tr.className = 'edit-row border-b border-slate-100 bg-amber-50/40';
            tr.setAttribute('data-id', id);
            tr.innerHTML = '<td colspan="6" class="py-3 px-4">'
              + '<form class="edit-template-form grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-3 items-end" data-id="' + window.qcsEscapeHTML(id) + '">'
              + miniField('Name', 'name', '<input type="text" name="name" value="' + window.qcsEscapeHTML(t.name || '') + '" class="w-full border border-slate-300 rounded px-2 py-1 text-sm">')
              + miniField('Service', 'service_type', '<select name="service_type" class="w-full border border-slate-300 rounded px-2 py-1 text-sm bg-white"><option value="standard"' + (t.service_type === 'standard' ? ' selected' : '') + '>Standard</option><option value="express"' + (t.service_type === 'express' ? ' selected' : '') + '>Express</option></select>')
              + miniField('Destination', 'destination_id', '<select name="destination_id" class="w-full border border-slate-300 rounded px-2 py-1 text-sm bg-white">' + destOptions(t.destination_id) + '</select>')
              + miniField('Recipient', 'recipient_id', '<select name="recipient_id" class="w-full border border-slate-300 rounded px-2 py-1 text-sm bg-white">' + recipOptions(recipId) + '</select>')
              + '<div class="flex items-center gap-2"><button type="submit" class="btn-save text-sm bg-[#2563EB] text-white px-3 py-1.5 rounded font-medium">Save</button>'
              + '<button type="button" class="btn-cancel text-sm text-slate-600 hover:underline">Cancel</button></div>'
              + '</form></td>';
            row.parentNode.insertBefore(tr, row.nextSibling);

            tr.querySelector('.edit-template-form').addEventListener('submit', function (ev) {
              ev.preventDefault();
              var fd = new FormData(ev.target);
              var payload = {
                name: fd.get('name'),
                service_type: fd.get('service_type'),
                destination_id: fd.get('destination_id'),
                recipient_id: fd.get('recipient_id') || null
              };
              QCS.fetchJson('/api/v1/templates/' + encodeURIComponent(id), { method: 'PATCH', body: payload })
                .then(function () { return QCS.fetchJson('/api/v1/templates'); })
                .then(function (j) {
                  state.list = (j && j.data) || [];
                  renderShell();
                  QCS.toast('Template updated', 'success');
                })
                .catch(function (err) { QCS.toast((err && err.message) || 'Update failed', 'error'); });
            });
            tr.querySelector('.btn-cancel').addEventListener('click', function () { tr.remove(); });
          });
        });
      }

      function miniField(label, name, control) {
        return '<div><label class="block text-xs font-medium text-slate-600 mb-1">' + window.qcsEscapeHTML(label) + '</label>' + control + '</div>';
      }
    })();
  