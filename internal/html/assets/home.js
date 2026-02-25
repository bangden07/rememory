var BUNDLES = {{BUNDLES_JSON}};

function formatDate(iso) {
  try {
    var d = new Date(iso);
    return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
  } catch { return iso; }
}

function escapeHtml(s) {
  var d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

function render() {
  var el = document.getElementById('content');
  if (BUNDLES.length === 0) {
    el.innerHTML =
      '<div class="empty-state">' +
        '<p>No recovery bundles here yet.</p>' +
        '<a href="maker.html" class="btn btn-primary">Create a bundle</a>' +
      '</div>';
    return;
  }

  var html = '';
  BUNDLES.forEach(function(b) {
    html +=
      '<div class="bundle-card" data-id="' + escapeHtml(b.id) + '">' +
        '<div class="bundle-date">' + formatDate(b.created) + '</div>' +
        '<div class="bundle-meta">' +
          b.threshold + ' of ' + b.total + ' pieces needed to recover' +
        '</div>' +
        '<div class="bundle-actions">' +
          '<a href="recover.html?id=' + encodeURIComponent(b.id) + '">Recover</a>' +
          '<button type="button" class="delete-toggle" onclick="toggleDelete(this)">Delete</button>' +
        '</div>' +
        '<div class="delete-form">' +
          '<input type="password" placeholder="Admin password" class="delete-password">' +
          '<button type="button" onclick="deleteBundle(this)" class="delete-btn">Confirm</button>' +
          '<div class="delete-error"></div>' +
        '</div>' +
      '</div>';
  });
  el.innerHTML = html;
}

function toggleDelete(btn) {
  var card = btn.closest('.bundle-card');
  var form = card.querySelector('.delete-form');
  form.classList.toggle('visible');
  if (form.classList.contains('visible')) {
    form.querySelector('.delete-password').focus();
  }
}

function deleteBundle(btn) {
  var card = btn.closest('.bundle-card');
  var id = card.dataset.id;
  var password = card.querySelector('.delete-password').value;
  var errorEl = card.querySelector('.delete-error');

  if (!password) {
    errorEl.textContent = 'Enter the admin password.';
    return;
  }

  btn.disabled = true;
  btn.textContent = 'Deleting...';
  errorEl.textContent = '';

  fetch('/api/bundle', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: id, password: password }),
  })
  .then(function(resp) {
    if (!resp.ok) return resp.text().then(function(t) { throw new Error(t || 'Delete failed.'); });
    BUNDLES = BUNDLES.filter(function(b) { return b.id !== id; });
    render();
  })
  .catch(function(err) {
    errorEl.textContent = err.message;
    btn.disabled = false;
    btn.textContent = 'Confirm';
  });
}

render();
