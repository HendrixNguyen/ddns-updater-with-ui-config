/* DDNS Updater web UI.
 *
 * No framework, no build step, no bundler: this file is shipped as is by
 * internal/server and embedded with `//go:embed ui/*`.
 *
 * SECURITY RULES OBSERVED THROUGHOUT
 *  - Server data (domains, provider labels, validation messages, raw JSON) is
 *    NEVER assigned through innerHTML or insertAdjacentHTML. Every node is
 *    built with document.createElement and filled with textContent.
 *  - Secret values (tokens, passwords, keys) never reach a data- attribute, a
 *    title tooltip, the URL or the console.
 *  - Every mutating request sends `Content-Type: application/json`.
 *  - Every fetch is bounded by an AbortController timeout so a hung request
 *    cannot wedge the page.
 *  - Raw JSON is never concatenated into a <script> block.
 */
(function () {
  'use strict';

  /* ------------------------------------------------------------ base path */

  var BASE = readBase();

  function readBase() {
    var value = window.__DDNS_BASE__;
    if (typeof value !== 'string') {
      return '';
    }
    return value.replace(/\/+$/, '');
  }

  function apiURL(path) {
    return BASE + '/api' + path;
  }

  function pageURL(path) {
    return BASE + '/' + path;
  }

  /* ---------------------------------------------------------- DOM helpers */

  var SVG_NS = 'http://www.w3.org/2000/svg';

  /* el(tag, props, children) creates an element. `props.text` sets
   * textContent, `props.html` is deliberately NOT supported, and unknown
   * props become attributes. `props.on*` become listeners. */
  function el(tag, props, children) {
    var node = document.createElement(tag);
    if (props) {
      Object.keys(props).forEach(function (key) {
        var value = props[key];
        if (value === null || value === undefined || value === false) {
          return;
        }
        if (key === 'text') {
          node.textContent = String(value);
        } else if (key === 'class') {
          node.className = String(value);
        } else if (key === 'dataset') {
          Object.keys(value).forEach(function (dataKey) {
            node.dataset[dataKey] = String(value[dataKey]);
          });
        } else if (key.slice(0, 2) === 'on') {
          node.addEventListener(key.slice(2).toLowerCase(), value);
        } else if (value === true) {
          node.setAttribute(key, '');
        } else {
          node.setAttribute(key, String(value));
        }
      });
    }
    appendChildren(node, children);
    return node;
  }

  function appendChildren(node, children) {
    if (children === null || children === undefined) {
      return;
    }
    if (!Array.isArray(children)) {
      children = [children];
    }
    children.forEach(function (child) {
      if (child === null || child === undefined || child === false) {
        return;
      }
      node.appendChild(typeof child === 'string' ? document.createTextNode(child) : child);
    });
  }

  function clear(node) {
    while (node.firstChild) {
      node.removeChild(node.firstChild);
    }
  }

  function byID(id) {
    return document.getElementById(id);
  }

  /* icon(paths, options) builds an inline SVG node from raw path data. The
   * path data is a hard coded constant of this file, never server data. */
  function icon(paths, options) {
    var opts = options || {};
    var svg = document.createElementNS(SVG_NS, 'svg');
    svg.setAttribute('viewBox', opts.viewBox || '0 0 24 24');
    svg.setAttribute('fill', opts.fill || 'none');
    svg.setAttribute('stroke', opts.fill ? 'none' : 'currentColor');
    svg.setAttribute('stroke-width', opts.strokeWidth || '1.8');
    svg.setAttribute('stroke-linecap', 'round');
    svg.setAttribute('stroke-linejoin', 'round');
    svg.setAttribute('aria-hidden', 'true');
    svg.setAttribute('class', opts.class || 'alert-icon');
    svg.setAttribute('width', opts.size || 20);
    svg.setAttribute('height', opts.size || 20);
    paths.forEach(function (d) {
      var path = document.createElementNS(SVG_NS, 'path');
      path.setAttribute('d', d);
      svg.appendChild(path);
    });
    return svg;
  }

  var ICONS = {
    success: ['M20 6 9 17l-5-5'],
    error: ['M12 8v5', 'M12 16.5v.01', 'M10.3 3.9 2.5 17.4A2 2 0 0 0 4.2 20.4h15.6a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z'],
    info: ['M12 16v-5', 'M12 8v.01', 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z'],
    warn: ['M12 9v4', 'M12 17v.01', 'M10.3 3.9 2.5 17.4A2 2 0 0 0 4.2 20.4h15.6a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z'],
    plus: ['M12 5v14', 'M5 12h14'],
    trash: ['M4 7h16', 'M10 11v6', 'M14 11v6', 'M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12', 'M9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2'],
    pencil: ['M12 20h9', 'M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z'],
    copy: ['M9 9h10a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2V11a2 2 0 0 1 2-2Z', 'M5 15H4a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v1'],
    refresh: ['M21 12a9 9 0 1 1-2.6-6.4', 'M21 3v6h-6'],
    search: ['M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16Z', 'm21 21-4.3-4.3'],
    chevron: ['m6 9 6 6 6-6'],
    close: ['M18 6 6 18', 'm6 6 12 12'],
    empty: ['M4 7h16v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2Z', 'M4 7l2-3h12l2 3', 'M9 12h6'],
  };

  /* -------------------------------------------------------------- fetching */

  var REQUEST_TIMEOUT_MS = 15000;

  function request(method, path, body) {
    var controller = new AbortController();
    var timer = window.setTimeout(function () {
      controller.abort();
    }, REQUEST_TIMEOUT_MS);
    var init = {
      method: method,
      signal: controller.signal,
      headers: { Accept: 'application/json' },
      credentials: 'same-origin',
      cache: 'no-store',
    };
    if (body !== undefined) {
      init.headers['Content-Type'] = 'application/json';
      init.body = JSON.stringify(body);
    }
    return fetch(path, init).then(function (response) {
      return response.text().then(function (text) {
        return { ok: response.ok, status: response.status, text: text };
      });
    }).then(function (result) {
      var data = null;
      if (result.text) {
        try {
          data = JSON.parse(result.text);
        } catch (e) {
          data = null;
        }
      }
      return { ok: result.ok, status: result.status, data: data, text: result.text };
    }).finally(function () {
      window.clearTimeout(timer);
    });
  }

  function getJSON(path) {
    return request('GET', path).then(function (result) {
      if (!result.ok) {
        throw new Error(errorsOf(result)[0] || ('request failed with status ' + result.status));
      }
      return result.data;
    });
  }

  /* errorsOf extracts the {errors: [...]} array of a failed response, falling
   * back to the raw body and then to a generic message. */
  function errorsOf(result) {
    var messages = [];
    if (result && result.data && Array.isArray(result.data.errors)) {
      result.data.errors.forEach(function (message) {
        if (typeof message === 'string' && message) {
          messages.push(message);
        }
      });
    }
    if (messages.length === 0 && result && result.data && typeof result.data.error === 'string') {
      messages.push(result.data.error);
    }
    if (messages.length === 0) {
      var status = result && result.status ? result.status : 0;
      messages.push(status ? ('the server answered with status ' + status) : 'the server could not be reached');
    }
    return messages;
  }

  /* ----------------------------------------------------------------- theme */

  function isDark() {
    return document.documentElement.classList.contains('dark');
  }

  function applyTheme(dark) {
    var root = document.documentElement;
    root.classList.toggle('dark', dark);
    root.classList.toggle('light', !dark);
    try {
      window.localStorage.setItem('ddns-theme', dark ? 'dark' : 'light');
    } catch (e) {
      /* storage unavailable, the choice simply does not persist */
    }
    var toggle = byID('theme-toggle');
    if (toggle) {
      toggle.setAttribute('aria-label', dark ? 'Switch to light theme' : 'Switch to dark theme');
      toggle.setAttribute('title', dark ? 'Switch to light theme' : 'Switch to dark theme');
    }
    var glyph = byID('theme-icon');
    if (glyph) {
      glyph.textContent = dark ? '☾' : '☀';
    }
  }

  function initTheme() {
    applyTheme(isDark());
    bindThemeToggle();
  }

  /* bindThemeToggle wires the header button, which on the settings page only
   * exists after the shell is built, hence the separate call and the guard. */
  function bindThemeToggle() {
    var toggle = byID('theme-toggle');
    if (!toggle || toggle.dataset.themeBound === 'true') {
      return;
    }
    toggle.dataset.themeBound = 'true';
    toggle.addEventListener('click', function () {
      applyTheme(!isDark());
    });
  }

  /* ---------------------------------------------------------------- toasts */

  var TOAST_MS = 5000;

  function toastRegion() {
    var region = byID('toasts');
    if (!region) {
      region = el('div', {
        class: 'toast-region',
        id: 'toasts',
        'aria-live': 'polite',
        'aria-atomic': 'false',
      });
      document.body.appendChild(region);
    }
    return region;
  }

  function toast(message, variant) {
    var kind = variant || 'info';
    var iconPaths = kind === 'success' ? ICONS.success : (kind === 'error' ? ICONS.error : ICONS.info);
    var node = el('div', { class: 'toast toast-' + kind }, [
      icon(iconPaths, { class: 'toast-icon' }),
      el('div', { class: 'toast-body', text: message }),
      el('button', {
        type: 'button',
        class: 'copy-btn',
        'aria-label': 'Dismiss notification',
        onClick: function () {
          remove();
        },
      }, [icon(ICONS.close, { class: 'alert-icon', strokeWidth: 2 })]),
    ]);
    var region = toastRegion();
    region.appendChild(node);
    var timer = window.setTimeout(remove, TOAST_MS);
    function remove() {
      window.clearTimeout(timer);
      if (node.parentNode) {
        node.parentNode.removeChild(node);
      }
    }
  }

  /* ---------------------------------------------------------------- modals */

  var FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]),' +
    ' select:not([disabled]), textarea:not([disabled]), summary, [tabindex]:not([tabindex="-1"])';

  var openModalHandle = null;

  function openModal(options) {
    closeModal();
    var previouslyFocused = document.activeElement;
    var titleID = 'modal-title-' + Date.now();
    var describedBy = options.describe ? (titleID + '-desc') : null;

    var body = el('div', { class: 'modal-body' }, options.body);
    var foot = options.footer ? el('div', { class: 'modal-foot' }, options.footer) : null;

    var dialog = el('div', {
      class: 'modal' + (options.small ? ' modal-sm drawer-enter' : ' drawer-enter'),
      role: 'dialog',
      'aria-modal': 'true',
      'aria-labelledby': titleID,
      'aria-describedby': describedBy,
    }, [
      el('div', { class: 'modal-head' }, [
        el('div', {}, [
          el('h2', { class: 'page-title', id: titleID, text: options.title }),
          options.subtitle ? el('p', { class: 'help', style: 'margin-top:0.25rem', text: options.subtitle }) : null,
        ]),
        el('button', {
          type: 'button',
          class: 'btn btn-sm btn-icon btn-ghost',
          'aria-label': 'Close dialog',
          onClick: function () {
            closeModal();
          },
        }, [icon(ICONS.close, { class: 'alert-icon', strokeWidth: 2 })]),
      ]),
      body,
      foot,
    ]);

    var overlay = el('div', {
      class: 'modal-overlay',
      onMousedown: function (event) {
        if (event.target === overlay) {
          closeModal();
        }
      },
    }, [dialog]);

    setBackgroundInert(true);
    document.body.appendChild(overlay);

    var handle = {
      overlay: overlay,
      dialog: dialog,
      close: closeModal,
    };
    openModalHandle = handle;

    document.addEventListener('keydown', onKeydown, true);
    var first = focusable(dialog)[0] || dialog;
    if (first === dialog) {
      dialog.setAttribute('tabindex', '-1');
    }
    first.focus();

    function onKeydown(event) {
      if (openModalHandle !== handle) {
        return;
      }
      if (event.key === 'Escape') {
        event.preventDefault();
        closeModal();
        return;
      }
      if (event.key !== 'Tab') {
        return;
      }
      var items = focusable(dialog);
      if (items.length === 0) {
        event.preventDefault();
        return;
      }
      var firstItem = items[0];
      var lastItem = items[items.length - 1];
      if (event.shiftKey && (document.activeElement === firstItem || document.activeElement === dialog)) {
        event.preventDefault();
        lastItem.focus();
      } else if (!event.shiftKey && document.activeElement === lastItem) {
        event.preventDefault();
        firstItem.focus();
      }
    }

    function restoreFocus() {
      setBackgroundInert(false);
      if (previouslyFocused && typeof previouslyFocused.focus === 'function' && document.contains(previouslyFocused)) {
        previouslyFocused.focus();
      }
    }

    function closeModal() {
      if (openModalHandle !== handle) {
        return;
      }
      openModalHandle = null;
      document.removeEventListener('keydown', onKeydown, true);
      if (overlay.parentNode) {
        overlay.parentNode.removeChild(overlay);
      }
      if (typeof options.onClose === 'function') {
        options.onClose();
      }
      restoreFocus();
    }

    return handle;
  }

  function focusable(root) {
    return Array.prototype.slice.call(root.querySelectorAll(FOCUSABLE))
      .filter(function (node) {
        return node.offsetParent !== null || node === document.activeElement;
      });
  }

  function setBackgroundInert(inert) {
    ['header.app-header', 'main.main', 'footer.footer'].forEach(function (selector) {
      var node = document.querySelector(selector);
      if (!node) {
        return;
      }
      if (inert) {
        node.setAttribute('inert', '');
        node.setAttribute('aria-hidden', 'true');
      } else {
        node.removeAttribute('inert');
        node.removeAttribute('aria-hidden');
      }
    });
  }

  /* ------------------------------------------------------------ formatting */

  var STATUS_META = {
    success: { label: 'Success', className: 'badge-success' },
    uptodate: { label: 'Up to date', className: 'badge-uptodate' },
    'up to date': { label: 'Up to date', className: 'badge-uptodate' },
    updating: { label: 'Updating', className: 'badge-updating' },
    failure: { label: 'Failure', className: 'badge-failure' },
    unset: { label: 'Unset', className: 'badge-unset' },
  };

  function statusBadge(status) {
    var key = typeof status === 'string' ? status.toLowerCase() : '';
    var meta = STATUS_META[key];
    if (!meta) {
      return el('span', { class: 'badge badge-neutral', text: status || 'Unknown' });
    }
    return el('span', { class: 'badge ' + meta.className, text: meta.label });
  }

  var MINUTE = 60;
  var HOUR = 60 * MINUTE;
  var DAY = 24 * HOUR;

  function relativeTime(value) {
    if (!value) {
      return 'never';
    }
    var time = new Date(value).getTime();
    if (isNaN(time)) {
      return 'unknown';
    }
    var deltaSeconds = Math.round((Date.now() - time) / 1000);
    if (deltaSeconds < 0) {
      deltaSeconds = 0;
    }
    if (deltaSeconds < 10) {
      return 'just now';
    }
    if (deltaSeconds < MINUTE) {
      return deltaSeconds + 's ago';
    }
    if (deltaSeconds < HOUR) {
      return Math.floor(deltaSeconds / MINUTE) + 'm ago';
    }
    if (deltaSeconds < DAY) {
      return Math.floor(deltaSeconds / HOUR) + 'h ago';
    }
    return Math.floor(deltaSeconds / DAY) + 'd ago';
  }

  function absoluteTime(value) {
    if (!value) {
      return 'never updated';
    }
    var time = new Date(value);
    if (isNaN(time.getTime())) {
      return 'unknown';
    }
    return time.toLocaleString();
  }

  function copyToClipboard(text) {
    if (!text) {
      return;
    }
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(function () {
        toast('Copied ' + text + ' to the clipboard', 'success');
      }, function () {
        toast('The address could not be copied', 'error');
      });
      return;
    }
    toast('Copying is not available in this browser', 'error');
  }

  /* busyState remembers the original children of every button set busy, so
   * that icons survive a busy/idle cycle. */
  var busyState = new WeakMap();

  function setBusy(button, busy, busyLabel) {
    if (busy) {
      if (busyState.has(button)) {
        return;
      }
      busyState.set(button, Array.prototype.slice.call(button.childNodes));
      clear(button);
      button.appendChild(el('span', { class: 'spinner', 'aria-hidden': 'true' }));
      button.appendChild(document.createTextNode(' ' + (busyLabel || 'Working')));
      button.setAttribute('aria-busy', 'true');
      button.setAttribute('disabled', '');
      return;
    }
    if (busyState.has(button)) {
      var original = busyState.get(button);
      busyState.delete(button);
      clear(button);
      original.forEach(function (node) {
        button.appendChild(node);
      });
    }
    button.removeAttribute('aria-busy');
    button.removeAttribute('disabled');
  }

  /* ---------------------------------------------------------------- header */

  function buildHeader(current) {
    var header = el('header', { class: 'app-header' }, [
      el('div', { class: 'container' }, [
        el('a', { class: 'brand', href: pageURL(''), 'aria-label': 'DDNS Updater home' }, [
          el('img', { src: pageURL('static/favicon.svg'), width: 28, height: 28, alt: '', 'aria-hidden': 'true' }),
          el('span', {}, [
            el('span', { text: 'DDNS Updater' }),
            el('span', { class: 'brand-sub', text: current === 'settings' ? 'Settings' : 'Records' }),
          ]),
        ]),
        el('nav', { class: 'app-nav', 'aria-label': 'Main' }, [
          el('a', {
            class: 'nav-link',
            href: pageURL(''),
            text: 'Records',
            'aria-current': current === 'records' ? 'page' : null,
          }),
          el('a', {
            class: 'nav-link',
            href: pageURL('settings'),
            text: 'Settings',
            'aria-current': current === 'settings' ? 'page' : null,
          }),
        ]),
        el('span', { class: 'spacer' }),
        el('button', {
          type: 'button',
          class: 'btn btn-sm btn-icon btn-ghost',
          id: 'theme-toggle',
          'aria-label': 'Switch theme',
          title: 'Switch theme',
        }, [el('span', { id: 'theme-icon', 'aria-hidden': 'true', text: isDark() ? '☾' : '☀' })]),
      ]),
    ]);
    return header;
  }

  function buildFooter() {
    return el('footer', { class: 'footer' }, [
      el('div', { class: 'container' }, [
        el('a', {
          href: 'https://github.com/qdm12/ddns-updater',
          class: 'text-big',
          'aria-label': 'DDNS Updater on GitHub',
          title: 'DDNS Updater on GitHub',
        }, [icon(['M8 0c4.42 0 8 3.58 8 8a8.013 8.013 0 0 1-5.45 7.59c-.4.08-.55-.17-.55-.38 0-.27.01-1.13.01-2.2'
            + ' 0-.75-.25-1.23-.54-1.48 1.78-.2 3.65-.88 3.65-3.95 0-.88-.31-1.59-.82-2.15.08-.2.36-1.02-.08-2.12 0 0-.67-.22-2.2.82-.64-.18-1.32-.27-2-.27-.68 0-1.36.09-2 .27-1.53-1.03-2.2-.82-2.2-.82-.44 1.1-.16 1.92-.08 2.12-.51.56-.82 1.28-.82 2.15'
            +             ' 0 3.06 1.86 3.75 3.64 3.95-.23.2-.44.55-.51 1.07-.46.21-1.61.55-2.33-.66-.15-.24-.6-.83-1.23-.82-.67.01-.27.38.01.53.34.19.73.9.82 1.13.16.45.68 1.31 2.69.94 0 .67.01 1.3.01 1.49 0 .21-.15.45-.55.38A7.995 7.995 0 0 1 0 8c0-4.42 3.58-8 8-8Z'],
          { class: 'github-icon', viewBox: '0 0 16 16', fill: 'currentColor', size: 18 })]),
        el('div', {}, [
          document.createTextNode('by '),
          el('a', { href: 'https://github.com/qdm12', text: 'Quentin McGaw' }),
          document.createTextNode(' / UI reworked by '),
          el('a', { href: 'https://github.com/fuse314', text: 'Gottfried Mayer' }),
        ]),
      ]),
    ]);
  }

  /* ======================================================= records page == */

  var POLL_INTERVAL_MS = 10000;
  var POLL_MAX_BACKOFF_MS = 60000;

  function initRecordsPage() {
    var body = byID('records-body');
    if (!body) {
      return;
    }
    var tableWrap = byID('records-table-wrap');
    var loading = byID('records-loading');
    var errorBox = byID('records-error');
    var errorDetail = byID('records-error-detail');
    var emptyBox = byID('records-empty');
    var connection = byID('connection');
    var retryButton = byID('records-retry');
    var forceButton = byID('force-update');

    var lastSignature = null;
    var failures = 0;
    var timer = null;
    var stopped = false;

    function setConnection(state, label) {
      if (!connection) {
        return;
      }
      connection.dataset.state = state;
      connection.textContent = label;
    }

    function showSkeleton() {
      loading.hidden = false;
      errorBox.hidden = true;
      emptyBox.hidden = true;
      tableWrap.hidden = true;
    }

    function showError(message) {
      loading.hidden = true;
      emptyBox.hidden = true;
      tableWrap.hidden = true;
      errorBox.hidden = false;
      errorDetail.textContent = message;
    }

    function showRecords(records) {
      loading.hidden = true;
      errorBox.hidden = true;
      emptyBox.hidden = records.length > 0;
      tableWrap.hidden = records.length === 0;
    }

    function renderStats(records) {
      var healthy = 0;
      var failing = 0;
      var providers = {};
      records.forEach(function (record) {
        var status = String(record.status || '').toLowerCase();
        if (status === 'success' || status === 'uptodate' || status === 'up to date') {
          healthy += 1;
        } else if (status === 'failure') {
          failing += 1;
        }
        if (record.provider) {
          providers[record.provider] = true;
        }
      });
      var total = String(records.length);
      var providerCount = String(Object.keys(providers).length);

      byID('stat-total').textContent = total;
      byID('stat-healthy').textContent = String(healthy);
      byID('stat-failing').textContent = String(failing);
      byID('stat-providers').textContent = providerCount;
      byID('stat-failing-hint').textContent = failing === 0 ? 'all good' : 'need attention';
      byID('stat-healthy-card').hidden = false;
      byID('stat-failing-card').hidden = false;
      byID('stat-providers-card').hidden = false;
    }

    function cell(label, content) {
      return el('td', { 'data-label': label }, content);
    }

    function previousIPsCell(record) {
      var ips = Array.isArray(record.previousIPs) ? record.previousIPs : [];
      if (ips.length === 0) {
        return cell('Previous IPs', [el('span', { class: 'subtle', text: 'none yet' })]);
      }
      var visible = ips.slice(0, 3);
      var hidden = ips.slice(3);
      var list = el('div', { class: 'pill-list' }, visible.map(function (ip) {
        return el('span', { class: 'mono', text: ip });
      }));
      if (hidden.length === 0) {
        return cell('Previous IPs', [list]);
      }
      var hiddenList = el('div', { class: 'pill-list', hidden: true },
        hidden.map(function (ip) {
          return el('span', { class: 'mono', text: ip });
        }));
      var toggle = el('button', {
        type: 'button',
        class: 'btn btn-sm btn-ghost',
        'aria-expanded': 'false',
        onClick: function () {
          var expanded = toggle.getAttribute('aria-expanded') === 'true';
          toggle.setAttribute('aria-expanded', expanded ? 'false' : 'true');
          hiddenList.hidden = expanded;
          clear(toggle);
          toggle.appendChild(document.createTextNode(expanded
            ? 'show all ' + ips.length
            : 'show less'));
        },
      }, [document.createTextNode('show all ' + ips.length)]);
      return cell('Previous IPs', [list, hiddenList, toggle]);
    }

    var MESSAGE_MAX_LENGTH = 180;

  /* messageNode keeps a provider failure short enough not to blow up the row
   * height. The full text is kept in the title attribute, as plain text. */
  function messageNode(message) {
    var full = String(message);
    var display = full;
    if (display.length > MESSAGE_MAX_LENGTH) {
      var clipped = display.slice(0, MESSAGE_MAX_LENGTH);
      var cut = clipped.lastIndexOf(' ');
      if (cut > MESSAGE_MAX_LENGTH * 0.6) {
        clipped = clipped.slice(0, cut);
      }
      display = clipped.replace(/[\s,;:.\-]+$/, '') + '…';
    }
    var node = el('div', {
      class: 'help msg-clamp',
      style: 'margin-top:0.25rem',
      text: display,
    });
    node.title = full;
    return node;
  }

  function renderRecords(records) {
      clear(body);
      records.forEach(function (record) {
        var domain = record.domain || '';
        var owner = record.owner || '';
        var ip = record.currentIP || '';
        var message = record.message || '';

        var statusCell = cell('Status', [
          statusBadge(record.status),
          message ? messageNode(message) : null,
        ]);

        var ipCell = cell('Current IP', [el('span', { class: 'ip-cell' }, [
          ip
            ? el('span', { class: 'mono', text: ip })
            : el('span', { class: 'subtle', text: 'N/A' }),
          ip
            ? el('button', {
              type: 'button',
              class: 'copy-btn',
              'aria-label': 'Copy the address of ' + domain,
              title: 'Copy to clipboard',
              onClick: function () {
                copyToClipboard(ip);
              },
            }, [icon(ICONS.copy, { class: 'alert-icon', strokeWidth: 1.8 })])
            : null,
        ])]);

        var updatedCell = cell('Last update', [el('span', {
          text: relativeTime(record.lastUpdate),
          title: absoluteTime(record.lastUpdate),
        })]);

        body.appendChild(el('tr', {}, [
          cell('Domain', [el('span', { class: 'domain-cell' }, [
            el('span', { class: 'mono', text: domain }),
            el('span', {
              class: 'domain-owner mono',
              text: owner ? 'owner ' + owner : 'owner derived from domain',
            }),
          ])]),
          cell('Provider', [el('span', { class: 'mono', text: record.provider || 'unknown' })]),
          cell('IP version', [el('span', { text: record.ipVersion || '—' })]),
          statusCell,
          ipCell,
          updatedCell,
          previousIPsCell(record),
        ]));
      });
    }

    function fetchRecords() {
      if (stopped) {
        return Promise.resolve();
      }
      if (document.hidden) {
        return Promise.resolve();
      }
      return getJSON(apiURL('/records')).then(function (payload) {
        var records = (payload && Array.isArray(payload.records)) ? payload.records : [];
        var signature = JSON.stringify(records);
        if (signature !== lastSignature) {
          lastSignature = signature;
          renderRecords(records);
          renderStats(records);
        }
        showRecords(records);
        failures = 0;
        setConnection('live', 'Live');
        schedule(POLL_INTERVAL_MS);
      }).catch(function (error) {
        failures += 1;
        var message = error && error.message ? error.message : 'the records API did not answer';
        if (lastSignature === null) {
          showError(message + '. Retrying automatically.');
        }
        setConnection('reconnecting', failures > 1 ? 'Reconnecting (' + failures + ')' : 'Reconnecting');
        var backoff = Math.min(POLL_MAX_BACKOFF_MS, 4000 * Math.pow(2, failures - 1));
        schedule(backoff);
      });
    }

    function schedule(delay) {
      if (timer) {
        window.clearTimeout(timer);
      }
      timer = window.setTimeout(function () {
        fetchRecords();
      }, delay);
    }

    function refreshNow() {
      if (timer) {
        window.clearTimeout(timer);
      }
      failures = 0;
      return fetchRecords();
    }

    if (retryButton) {
      retryButton.addEventListener('click', function () {
        showSkeleton();
        refreshNow();
      });
    }

    if (forceButton) {
      forceButton.addEventListener('click', function (event) {
        event.preventDefault();
        setBusy(forceButton, true, 'Updating');
        request('GET', pageURL('update')).then(function (result) {
          if (result.ok || result.status === 202) {
            toast('Update forced for every record', 'success');
            refreshNow();
          } else {
            toast('Update failed: ' + errorsOf(result).join(' '), 'error');
          }
        }).catch(function () {
          toast('The update request could not be sent', 'error');
        }).finally(function () {
          setBusy(forceButton, false);
        });
      });
    }

    document.addEventListener('visibilitychange', function () {
      if (document.hidden) {
        if (timer) {
          window.clearTimeout(timer);
          timer = null;
        }
        return;
      }
      refreshNow();
    });

    window.addEventListener('pagehide', function () {
      stopped = true;
      if (timer) {
        window.clearTimeout(timer);
      }
    });

    showSkeleton();
    refreshNow();
  }

  /* ===================================================== settings page == */

  var COMMON_KEYS = ['provider', 'domain', 'owner', 'ip_version', 'ipv6_suffix', 'host', 'provider_ip'];
  var SECRET_KEY_PATTERN = /token|password|secret|key/i;
  var DOC_BASE_URL = 'https://github.com/qdm12/ddns-updater/blob/main/';
  var IP_VERSION_DEFAULT = 'ipv4 or ipv6';
  var IP_VERSION_VALUES = ['ipv4', 'ipv6', IP_VERSION_DEFAULT];
  var IP_VERSION_EMPTY = '';

  /* ipVersionLabel turns the raw server string into a human label, and turns
   * a missing value into the "use the server default" wording. */
  function ipVersionLabel(value) {
    if (typeof value !== 'string' || value.trim() === '') {
      return 'Default (IPv4 and IPv6)';
    }
    var normalized = value.trim().toLowerCase();
    if (normalized === 'ipv4') {
      return 'IPv4';
    }
    if (normalized === 'ipv6') {
      return 'IPv6';
    }
    if (normalized === IP_VERSION_DEFAULT) {
      return 'IPv4 and IPv6';
    }
    return value;
  }

  function isKnownIPVersion(value) {
    return IP_VERSION_VALUES.indexOf(String(value).trim().toLowerCase()) !== -1;
  }

  function initSettingsPage() {
    var app = byID('app');
    if (!app) {
      return;
    }

    var providers = [];
    var settings = null;
    var search = '';
    var activeTab = 'form';
    var rawDirty = false;
    var loadError = null;

    var nodes = {};

    buildShell();

    getJSON(apiURL('/providers')).then(function (payload) {
      providers = (payload && Array.isArray(payload.providers)) ? payload.providers : [];
    }).catch(function () {
      providers = [];
      toast('The provider list could not be loaded, provider fields are unavailable', 'error');
    });

    refresh();

    /* ------------------------------------------------------------- shell */

    function buildShell() {
      var main = el('main', { class: 'main', id: 'settings-main' }, [
        el('div', { class: 'container stack' }),
      ]);
      var stack = main.firstChild;

      var addButton = el('button', {
        type: 'button',
        class: 'btn btn-primary',
        onClick: function () {
          openEntryModal(null);
        },
      }, [icon(ICONS.plus, { class: 'alert-icon', strokeWidth: 2 }), document.createTextNode('Add record')]);

      var reloadButton = el('button', {
        type: 'button',
        class: 'btn',
        onClick: reload,
      }, [icon(ICONS.refresh, { class: 'alert-icon' }), document.createTextNode('Reload')]);

      nodes.addButton = addButton;
      nodes.reloadButton = reloadButton;
      nodes.banner = el('section', { 'aria-label': 'Configuration source' });
      nodes.alerts = el('section', { 'aria-label': 'Configuration warnings' });
      nodes.listCard = el('section', { class: 'card', 'aria-labelledby': 'records-heading' });
      nodes.jsonCard = el('section', { class: 'card', 'aria-labelledby': 'json-heading', hidden: true });

      buildEditor();

      stack.appendChild(el('section', { class: 'row', style: 'align-items:flex-end' }, [
        el('div', {}, [
          el('h1', { class: 'page-title', text: 'Settings' }),
          el('p', {
            class: 'page-lede',
            text: 'Add, edit and remove the DDNS records the updater keeps in sync. Changes are written to ' +
              'the settings file and reloaded automatically.',
          }),
        ]),
        el('span', { class: 'spacer' }),
        nodes.tabs,
        reloadButton,
        addButton,
      ]));

      stack.appendChild(nodes.banner);
      stack.appendChild(nodes.alerts);
      stack.appendChild(nodes.formPanel);
      stack.appendChild(nodes.jsonCard);

      app.appendChild(el('a', { class: 'sr-only', href: '#settings-main', text: 'Skip to content' }));
      app.appendChild(buildHeader('settings'));
      bindThemeToggle();
      app.appendChild(main);
      app.appendChild(buildFooter());
      app.appendChild(el('div', { class: 'toast-region', id: 'toasts', 'aria-live': 'polite' }));
    }

    /* buildEditor creates the "Form" / "Raw JSON" tabs and their two panels.
     * The Form panel hosts the list of configured records, the Raw JSON panel
     * hosts the whole document editor. */
    function buildEditor() {
      var textarea = el('textarea', {
        class: 'textarea textarea-code',
        id: 'raw-json',
        spellcheck: 'false',
        wrap: 'off',
        rows: '18',
        'aria-label': 'Raw settings JSON document',
        onInput: function () {
          rawDirty = true;
          updateJSONState();
        },
        onKeydown: function (event) {
          if (event.key === 'Tab') {
            event.preventDefault();
            var start = textarea.selectionStart;
            var end = textarea.selectionEnd;
            textarea.value = textarea.value.slice(0, start) + '  ' + textarea.value.slice(end);
            textarea.selectionStart = textarea.selectionStart + 2;
            rawDirty = true;
            updateJSONState();
          }
        },
      });
      nodes.rawTextarea = textarea;

      var status = el('p', { class: 'help', id: 'raw-status', role: 'status' });
      var errors = el('div', { id: 'raw-errors' });
      nodes.rawStatus = status;
      nodes.rawErrors = errors;

      var validateButton = el('button', {
        type: 'button',
        class: 'btn',
        onClick: validateRaw,
      }, [document.createTextNode('Validate')]);
      var saveButton = el('button', {
        type: 'button',
        class: 'btn btn-primary',
        onClick: saveRaw,
      }, [document.createTextNode('Save all')]);
      nodes.rawValidateButton = validateButton;
      nodes.rawSaveButton = saveButton;

      var formPanel = el('div', {
        class: 'stack',
        id: 'panel-form',
        role: 'tabpanel',
        'aria-labelledby': 'tab-form',
      }, [nodes.listCard]);

      var jsonPanel = el('div', {
        class: 'stack',
        id: 'panel-json',
        role: 'tabpanel',
        'aria-labelledby': 'tab-json',
        hidden: true,
      }, [
        el('div', { class: 'card-body stack' }, [
          el('div', {}, [
            el('label', { class: 'sr-only', for: 'raw-json', text: 'Settings document' }),
            textarea,
            status,
            errors,
          ]),
          el('div', { class: 'row row-end' }, [validateButton, saveButton]),
        ]),
      ]);

      nodes.formPanel = formPanel;
      nodes.jsonPanel = jsonPanel;

      var tabs = el('div', { class: 'tabs', role: 'tablist', 'aria-label': 'Editing mode' }, [
        el('button', {
          type: 'button',
          class: 'tab',
          id: 'tab-form',
          role: 'tab',
          'aria-selected': 'true',
          'aria-controls': 'panel-form',
          text: 'Form',
          onClick: function () {
            selectTab('form');
          },
          onKeydown: onTabKey,
        }),
        el('button', {
          type: 'button',
          class: 'tab',
          id: 'tab-json',
          role: 'tab',
          'aria-selected': 'false',
          'aria-controls': 'panel-json',
          tabindex: '-1',
          text: 'Raw JSON',
          onClick: function () {
            selectTab('json');
          },
          onKeydown: onTabKey,
        }),
      ]);
      nodes.tabs = tabs;

      nodes.jsonCard.appendChild(el('div', { class: 'card-head' }, [
        el('div', {}, [
          el('h2', { id: 'json-heading', text: 'Raw settings document' }),
          el('p', { class: 'help', style: 'margin-top:0.25rem', text: 'The exact JSON written to the settings ' +
            'file. Top level fields other than "settings" are preserved by the server when it writes it back.' }),
        ]),
      ]));
      nodes.jsonCard.appendChild(jsonPanel);
    }

    function onTabKey(event) {
      var order = ['form', 'json'];
      var index = order.indexOf(activeTab);
      if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
        event.preventDefault();
        selectTab(order[(index + 1) % order.length], true);
      } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
        event.preventDefault();
        selectTab(order[(index + order.length - 1) % order.length], true);
      } else if (event.key === 'Home') {
        event.preventDefault();
        selectTab(order[0], true);
      } else if (event.key === 'End') {
        event.preventDefault();
        selectTab(order[order.length - 1], true);
      }
    }

    function selectTab(name, focusTab) {
      activeTab = name;
      var isForm = name === 'form';
      nodes.formPanel.hidden = !isForm;
      nodes.jsonPanel.hidden = isForm;
      nodes.jsonCard.hidden = isForm;
      var formTab = nodes.tabs.querySelector('#tab-form');
      var jsonTab = nodes.tabs.querySelector('#tab-json');
      formTab.setAttribute('aria-selected', isForm ? 'true' : 'false');
      jsonTab.setAttribute('aria-selected', isForm ? 'false' : 'true');
      formTab.tabIndex = isForm ? 0 : -1;
      jsonTab.tabIndex = isForm ? -1 : 0;
      if (focusTab) {
        (isForm ? formTab : jsonTab).focus();
      }
      if (isForm) {
        renderList();
      } else if (!rawDirty) {
        syncRawFromSettings();
      }
    }

    /* --------------------------------------------------------- data load */

    function refresh() {
      return getJSON(apiURL('/settings')).then(function (payload) {
        settings = payload || { settings: [], warnings: [] };
        loadError = null;
        applySettings();
      }).catch(function (error) {
        loadError = error && error.message ? error.message : 'the settings API did not answer';
        applySettings();
      });
    }

    function applySettings() {
      if (!Array.isArray(settings.settings)) {
        settings.settings = [];
      }
      if (!Array.isArray(settings.warnings)) {
        settings.warnings = [];
      }
      renderBanner();
      renderWarnings();
      renderList();
      if (!rawDirty) {
        syncRawFromSettings();
      } else {
        updateJSONState();
      }
    }

    function renderBanner() {
      clear(nodes.banner);
      if (loadError) {
        nodes.banner.appendChild(el('div', { class: 'alert alert-error', role: 'alert' }, [
          icon(ICONS.error, {}),
          el('div', { class: 'alert-body' }, [
            el('p', { class: 'alert-title', text: 'Settings could not be read' }),
            el('p', { class: 'muted', text: loadError }),
          ]),
          el('button', {
            type: 'button',
            class: 'btn btn-sm',
            text: 'Retry',
            onClick: function () {
              refresh();
            },
          }),
        ]));
        return;
      }

      var source = settings.source || 'empty';
      var env = settings.env || 'unset';
      var sourceBadge = el('span', {
        class: 'badge ' + (source === 'file' ? 'badge-success' : (source === 'env' ? 'badge-updating' : 'badge-neutral')),
        text: 'source: ' + source,
      });
      var envBadge = el('span', {
        class: 'badge ' + (env === 'set' ? 'badge-accent' : 'badge-neutral badge-plain'),
        text: 'CONFIG: ' + env,
      });

      nodes.banner.appendChild(el('div', { class: 'card card-pad' }, [
        el('div', { class: 'row', style: 'align-items:flex-start' }, [
          el('div', { style: 'flex:1 1 20rem;min-width:0' }, [
            el('div', { class: 'stat-label', text: 'Settings file' }),
            el('p', { class: 'mono truncate', style: 'margin-top:0.25rem', text: settings.filePath || 'unknown' }),
            el('p', { class: 'help', style: 'margin-top:0.5rem' }, [
              document.createTextNode('This page writes to that file. '),
              document.createTextNode(String(settings.records || 0) + ' record(s) currently running.'),
            ]),
          ]),
          el('div', { class: 'row', style: 'gap:0.5rem' }, [sourceBadge, envBadge]),
        ]),
      ]));

      if (source === 'env') {
        nodes.banner.appendChild(el('div', {
          class: 'alert alert-warn',
          role: 'note',
          style: 'margin-top:0.75rem',
        }, [
          icon(ICONS.warn, {}),
          el('div', { class: 'alert-body' }, [
            el('p', { class: 'alert-title', text: 'The CONFIG environment variable wins' }),
            el('p', { style: 'margin-top:0.25rem', text: 'The CONFIG environment variable currently overrides this ' +
              'file. Changes you make here are saved to disk and are used as long as CONFIG is unchanged in your ' +
              'compose file; if you edit CONFIG there, it wins again.' }),
          ]),
        ]));
      } else if (source === 'file' && env === 'set') {
        nodes.banner.appendChild(el('div', {
          class: 'alert alert-info',
          role: 'note',
          style: 'margin-top:0.75rem',
        }, [
          icon(ICONS.info, {}),
          el('div', { class: 'alert-body' }, [
            el('p', { class: 'alert-title', text: 'The settings file is in effect' }),
            el('p', { style: 'margin-top:0.25rem', text: 'The CONFIG environment variable is set but unchanged ' +
              'since the last start, so the settings file (including your changes) is in effect.' }),
          ]),
        ]));
      }
    }

    function renderWarnings() {
      clear(nodes.alerts);
      settings.warnings.forEach(function (warning) {
        var node = el('div', { class: 'alert alert-warn', role: 'note', style: 'margin-bottom:0.75rem' }, [
          icon(ICONS.warn, {}),
          el('div', { class: 'alert-body' }, [el('p', { text: warning })]),
          el('button', {
            type: 'button',
            class: 'copy-btn',
            'aria-label': 'Dismiss this warning',
            onClick: function () {
              if (node.parentNode) {
                node.parentNode.removeChild(node);
              }
            },
          }, [icon(ICONS.close, { class: 'alert-icon', strokeWidth: 2 })]),
        ]);
        nodes.alerts.appendChild(node);
      });
    }

    /* -------------------------------------------------------- entries list */

    function providerDocURL(name) {
      var provider = findProvider(name);
      if (!provider || !provider.doc) {
        return null;
      }
      return DOC_BASE_URL + provider.doc;
    }

    function findProvider(name) {
      for (var i = 0; i < providers.length; i++) {
        if (providers[i].name === name) {
          return providers[i];
        }
      }
      return null;
    }

    function entryDomain(entry) {
      return entry && typeof entry.domain === 'string' ? entry.domain : '';
    }

    function entryProvider(entry) {
      return entry && typeof entry.provider === 'string' ? entry.provider : '';
    }

    function otherKeys(entry) {
      return Object.keys(entry).filter(function (key) {
        return COMMON_KEYS.indexOf(key) === -1;
      });
    }

    function chipFor(key, value) {
      if (value === null || value === undefined || value === '') {
        return el('span', { class: 'badge badge-plain', text: key });
      }
      if (typeof value === 'object') {
        return el('span', { class: 'badge badge-plain', text: key + ' {…}' });
      }
      var text = SECRET_KEY_PATTERN.test(key)
        ? key + '=••••••'
        : key + '=' + String(value);
      if (text.length > 40) {
        text = text.slice(0, 39) + '…';
      }
      return el('span', { class: 'badge badge-plain', title: key, text: text });
    }

    function renderList() {
      clear(nodes.listCard);
      if (!settings) {
        return;
      }
      if (loadError) {
        nodes.listCard.appendChild(el('div', { class: 'empty' }, [
          icon(ICONS.error, { class: 'empty-icon' }),
          el('h2', { id: 'records-heading', text: 'Settings unavailable' }),
          el('p', { text: loadError }),
          el('button', { type: 'button', class: 'btn btn-primary', text: 'Retry', onClick: refresh }),
        ]));
        return;
      }

      var entries = settings.settings;
      var head = el('div', { class: 'card-head' }, [
        el('div', {}, [
          el('h2', { id: 'records-heading', text: 'Configured records' }),
          el('p', {
            class: 'help',
            style: 'margin-top:0.25rem',
            text: entries.length + (entries.length === 1 ? ' record' : ' records') + ' in the settings document.',
          }),
        ]),
      ]);

      if (entries.length > 8) {
        head.appendChild(el('label', { class: 'row', style: 'gap:0.5rem;flex:0 0 auto' }, [
          icon(ICONS.search, { class: 'alert-icon' }),
          el('span', { class: 'sr-only', text: 'Filter records' }),
          el('input', {
            class: 'input',
            type: 'search',
            placeholder: 'Filter records…',
            value: search,
            style: 'width:14rem;min-height:2rem',
            onInput: function (event) {
              search = event.target.value;
              renderRows();
            },
          }),
        ]));
      }
      nodes.listCard.appendChild(head);

      var bodyWrap = el('div', {});
      nodes.listBody = bodyWrap;
      nodes.listCard.appendChild(bodyWrap);
      renderRows();
    }

    function matchesSearch(entry) {
      if (!search) {
        return true;
      }
      var needle = search.toLowerCase();
      return JSON.stringify(entry).toLowerCase().indexOf(needle) !== -1;
    }

    function renderRows() {
      if (!nodes.listBody || !settings) {
        return;
      }
      clear(nodes.listBody);
      if (loadError) {
        return;
      }
      var entries = settings.settings;

      if (entries.length === 0) {
        nodes.listBody.appendChild(el('div', { class: 'empty' }, [
          icon(ICONS.empty, { class: 'empty-icon' }),
          el('h2', { text: 'No record configured yet' }),
          el('p', { text: 'Add a domain and the credentials of your DNS provider, and the updater will keep it ' +
            'pointing at this machine.' }),
          el('button', {
            type: 'button',
            class: 'btn btn-primary',
            onClick: function () {
              openEntryModal(null);
            },
          }, [icon(ICONS.plus, { class: 'alert-icon', strokeWidth: 2 }), document.createTextNode('Add the first record')]),
        ]));
        return;
      }

      var visible = entries.map(function (entry, index) {
        return { entry: entry, index: index };
      }).filter(function (item) {
        return matchesSearch(item.entry);
      });

      if (visible.length === 0) {
        nodes.listBody.appendChild(el('div', { class: 'empty' }, [
          icon(ICONS.search, { class: 'empty-icon' }),
          el('h2', { text: 'No record matches the filter' }),
          el('p', { text: 'Clear the filter to see all ' + entries.length + ' records.' }),
        ]));
        return;
      }

      var tbody = el('tbody', {});
      visible.forEach(function (item) {
        var entry = item.entry;
        var providerName = entryProvider(entry);
        var docURL = providerDocURL(providerName);
        var keys = otherKeys(entry);

        var providerCell = el('td', { 'data-label': 'Provider' }, [
          el('div', { class: 'row', style: 'gap:0.375rem' }, [
            el('span', { class: 'mono', text: providerName || 'unknown' }),
            docURL
              ? el('a', {
                class: 'subtle',
                href: docURL,
                target: '_blank',
                rel: 'noreferrer noopener',
                text: 'docs ↗',
                'aria-label': 'Documentation of the ' + (providerName || 'unknown') + ' provider',
                style: 'font-size:0.75rem',
              })
              : null,
          ]),
        ]);

        var summaryCell = el('td', { 'data-label': 'Options' }, [
          keys.length === 0
            ? el('span', { class: 'subtle', text: 'defaults' })
            : el('div', { class: 'row', style: 'gap:0.25rem' }, keys.map(function (key) {
              return chipFor(key, entry[key]);
            })),
        ]);

        var actions = el('td', { class: 'actions', 'data-label': 'Actions' }, [
          el('div', { class: 'row row-end', style: 'gap:0.375rem' }, [
            el('button', {
              type: 'button',
              class: 'btn btn-sm',
              'aria-label': 'Edit ' + (entryDomain(entry) || 'this record'),
              onClick: function () {
                openEntryModal(item.index);
              },
            }, [icon(ICONS.pencil, { class: 'alert-icon' }), document.createTextNode('Edit')]),
            el('button', {
              type: 'button',
              class: 'btn btn-sm btn-danger',
              'aria-label': 'Delete ' + (entryDomain(entry) || 'this record'),
              onClick: function () {
                confirmDelete(item.index);
              },
            }, [icon(ICONS.trash, { class: 'alert-icon' }), document.createTextNode('Delete')]),
          ]),
        ]);

        tbody.appendChild(el('tr', {}, [
          el('td', { 'data-label': '#' }, [el('span', { class: 'mono subtle', text: String(item.index) })]),
          providerCell,
          el('td', { 'data-label': 'Domain' }, [el('span', { class: 'mono', text: entryDomain(entry) })]),
          el('td', { 'data-label': 'Owner' }, [el('span', {
            class: 'mono',
            text: typeof entry.owner === 'string' && entry.owner ? entry.owner : 'derived',
          })]),
          el('td', { 'data-label': 'IP version' }, [el('span', {
            title: typeof entry.ip_version === 'string' && entry.ip_version
              ? String(entry.ip_version)
              : 'not set, the server default applies',
            text: ipVersionLabel(entry.ip_version),
          })]),
          summaryCell,
          actions,
        ]));
      });

      nodes.listBody.appendChild(el('div', { class: 'table-wrap' }, [
        el('table', { class: 'table' }, [
          el('caption', { class: 'sr-only', text: 'Settings entries, with their provider, domain and actions.' }),
          el('thead', {}, [el('tr', {}, [
            el('th', { scope: 'col', text: '#' }),
            el('th', { scope: 'col', text: 'Provider' }),
            el('th', { scope: 'col', text: 'Domain' }),
            el('th', { scope: 'col', text: 'Owner' }),
            el('th', { scope: 'col', text: 'IP version' }),
            el('th', { scope: 'col', text: 'Options' }),
            el('th', { scope: 'col', class: 'actions', text: 'Actions' }),
          ])]),
          tbody,
        ]),
      ]));
    }

    /* ------------------------------------------------------------ mutations */

    function adoptSettings(payload) {
      if (payload && typeof payload === 'object') {
        settings = payload;
        rawDirty = false;
        applySettings();
      }
    }

    function reload() {
      setBusy(nodes.reloadButton, true, 'Reloading');
      request('POST', apiURL('/reload')).then(function (result) {
        if (!result.ok) {
          toast('Reload failed: ' + errorsOf(result).join(' '), 'error');
          return null;
        }
        var reloaded = result.data || {};
        toast('Configuration reloaded, ' + (reloaded.records || 0) + ' record(s) running', 'success');
        return refresh();
      }).catch(function () {
        toast('The reload request could not be sent', 'error');
      }).finally(function () {
        setBusy(nodes.reloadButton, false);
      });
    }

    function confirmDelete(index) {
      var entry = settings.settings[index];
      if (!entry) {
        return;
      }
      var domain = entryDomain(entry);
      var name = domain || ('entry #' + index);
      var confirmButton = el('button', {
        type: 'button',
        class: 'btn btn-danger-solid',
        text: 'Delete record',
        onClick: function () {
          setBusy(confirmButton, true, 'Deleting');
          request('DELETE', apiURL('/settings/' + encodeURIComponent(String(index))))
            .then(function (result) {
              if (!result.ok) {
                toast('Delete failed: ' + errorsOf(result).join(' '), 'error');
                return;
              }
              handle.close();
              adoptSettings(result.data);
              toast('Deleted ' + name, 'success');
            })
            .catch(function () {
              toast('The delete request could not be sent', 'error');
            })
            .finally(function () {
              setBusy(confirmButton, false);
            });
        },
      });

      var handle = openModal({
        title: 'Delete this record?',
        subtitle: name,
        small: true,
        describe: false,
        body: [
          el('p', { text: 'The entry for ' + (domain ? 'the domain ' + domain : 'entry #' + index) +
            ' will be removed from the settings file and the updater will stop updating it. This cannot be undone.' }),
        ],
        footer: [
          el('button', { type: 'button', class: 'btn btn-ghost', text: 'Cancel', onClick: function () {
            handle.close();
          } }),
          confirmButton,
        ],
      });
    }

    /* --------------------------------------------------------- entry modal */

    function openEntryModal(index, draftOverride) {
      var isEdit = typeof index === 'number' && index >= 0;
      var entry = isEdit ? settings.settings[index] : (draftOverride || {});
      if (!entry || typeof entry !== 'object' || Array.isArray(entry)) {
        entry = {};
      }
      var draft = {};
      Object.keys(entry).forEach(function (key) {
        draft[key] = entry[key];
      });

      var formErrorBox = el('div', { class: 'alert alert-error', hidden: true, role: 'alert' });
      var validationBox = el('div', { 'aria-live': 'polite' });
      var fieldsHost = el('div', {});
      var advancedBody = el('div', { class: 'advanced-body' });
      var providerLink = el('a', {
        class: 'help',
        target: '_blank',
        rel: 'noreferrer noopener',
        text: 'Provider documentation ↗',
        hidden: true,
      });

      var providerInput = el('input', {
        class: 'input',
        type: 'text',
        id: 'entry-provider',
        role: 'combobox',
        autocomplete: 'off',
        spellcheck: 'false',
        'aria-expanded': 'false',
        'aria-autocomplete': 'list',
        'aria-controls': 'entry-provider-list',
        placeholder: 'Search a provider…',
        style: 'padding-right:2.25rem',
        onInput: function () {
          openProviderList();
        },
        onClick: function () {
          openProviderList();
        },
        onFocus: function () {
          openProviderList();
        },
        onKeydown: onProviderKey,
      });
      /* The caret is pure decoration: its geometry, colour and the click
       * pass-through all live in the `.combobox-caret` rule of styles.css,
       * and only the open/closed rotation is toggled, via `is-open`. */
      var providerCaret = el('span', {
        class: 'combobox-caret',
        'aria-hidden': 'true',
      }, [icon(ICONS.chevron, { class: 'combobox-caret-icon', strokeWidth: 2 })]);
      var providerList = el('ul', {
        class: 'combobox-list',
        id: 'entry-provider-list',
        role: 'listbox',
        hidden: true,
        'aria-label': 'Providers',
      });
      var providerBox = el('div', { class: 'combobox' }, [providerInput, providerCaret, providerList]);
      var filtered = [];
      var activeOption = -1;

      function onDocumentPointerDown(event) {
        if (!providerBox.contains(event.target)) {
          closeProviderList();
        }
      }
      document.addEventListener('mousedown', onDocumentPointerDown, true);
      document.addEventListener('touchstart', onDocumentPointerDown, true);

      var domainInput = el('input', {
        class: 'input',
        type: 'text',
        id: 'entry-domain',
        required: true,
        autocomplete: 'off',
        spellcheck: 'false',
        placeholder: '*.example.com or home.office.example.com',
        value: typeof draft.domain === 'string' ? draft.domain : '',
        onInput: function () {
          scheduleValidation();
        },
      });
      var ownerInput = el('input', {
        class: 'input',
        type: 'text',
        id: 'entry-owner',
        autocomplete: 'off',
        spellcheck: 'false',
        placeholder: '@',
        value: typeof draft.owner === 'string' ? draft.owner : '',
        onInput: function () {
          scheduleValidation();
        },
      });

      var ipVersionSelect = el('select', {
        class: 'select',
        id: 'entry-ip-version',
        onChange: function () {
          scheduleValidation();
        },
      }, [
        el('option', { value: IP_VERSION_EMPTY, text: 'Default (IPv4 and IPv6)' }),
        el('option', { value: IP_VERSION_DEFAULT, text: 'IPv4 and IPv6' }),
        el('option', { value: 'ipv4', text: 'IPv4 only' }),
        el('option', { value: 'ipv6', text: 'IPv6 only' }),
      ]);
      ipVersionSelect.value = isKnownIPVersion(draft.ip_version)
        ? String(draft.ip_version).trim().toLowerCase()
        : IP_VERSION_EMPTY;

      var suffixInput = el('input', {
        class: 'input',
        type: 'text',
        id: 'entry-ipv6-suffix',
        placeholder: '::/64',
        spellcheck: 'false',
        autocomplete: 'off',
        value: typeof draft.ipv6_suffix === 'string' ? draft.ipv6_suffix : '',
        onInput: function () {
          validateSuffix();
          scheduleValidation();
        },
      });
      var suffixError = el('p', { class: 'error-text', hidden: true });

      var jsonArea = el('textarea', {
        class: 'textarea textarea-code',
        id: 'entry-json',
        rows: '8',
        spellcheck: 'false',
        wrap: 'off',
        placeholder: '{\n  "token": "…"\n}',
        onInput: function () {
          validateJSONArea();
          scheduleValidation();
        },
      });
      var jsonError = el('p', { class: 'error-text', hidden: true });

      var fieldRefs = [];
      var selectedProvider = null;
      var validationTimer = null;
      var validationSeq = 0;

      function setProvider(name) {
        var provider = findProvider(name);
        if (selectedProvider !== null && selectedProvider !== name) {
          /* The provider changed, so the options of the previous one must not
           * leak into the new entry. Unknown keys are kept when the provider
           * has no curated field, since the free form JSON editor owns them. */
          var allowed = {};
          var newFields = (provider && Array.isArray(provider.fields)) ? provider.fields : [];
          newFields.forEach(function (field) {
            allowed[field.key] = true;
          });
          if (newFields.length > 0) {
            Object.keys(draft).forEach(function (key) {
              if (COMMON_KEYS.indexOf(key) === -1 && !allowed[key]) {
                delete draft[key];
              }
            });
          }
        }
        selectedProvider = name;
        draft.provider = name;
        providerInput.value = provider ? provider.label : name;
        if (provider && provider.doc) {
          providerLink.hidden = false;
          providerLink.href = DOC_BASE_URL + provider.doc;
          providerLink.textContent = (provider.label || name) + ' documentation ↗';
        } else {
          providerLink.hidden = true;
          providerLink.removeAttribute('href');
        }
        closeProviderList();
        renderProviderFields(provider);
        scheduleValidation();
      }

      function renderProviderFields(provider) {
        fieldRefs = [];
        clear(fieldsHost);
        var fields = (provider && Array.isArray(provider.fields)) ? provider.fields : [];
        if (fields.length === 0) {
          var remaining = {};
          Object.keys(draft).forEach(function (key) {
            if (COMMON_KEYS.indexOf(key) === -1) {
              remaining[key] = draft[key];
            }
          });
          jsonArea.value = Object.keys(remaining).length > 0
            ? JSON.stringify(remaining, null, 2)
            : '';
          fieldsHost.appendChild(el('div', { class: 'field' }, [
            el('label', { class: 'label', for: 'entry-json' }, [
              document.createTextNode('Provider options '),
              el('span', { class: 'subtle', style: 'font-weight:400', text: '(JSON object)' }),
            ]),
            jsonArea,
            el('p', { class: 'help', text: 'This provider has no curated field, so its options are edited as ' +
              'JSON. Keys such as "token", "username" or "ttl" are passed to the provider as they are. Leave the ' +
              'object empty to use the defaults.' }),
            jsonError,
          ]));
          return;
        }
        fields.forEach(function (field) {
          fieldsHost.appendChild(renderField(field));
        });
      }

      function renderField(field) {
        var inputID = 'entry-field-' + field.key;
        var errorNode = el('p', { class: 'error-text', hidden: true });
        var control;
        var current = draft[field.key];
        if (current === undefined || current === null) {
          current = field.type === 'checkbox' ? false : '';
        }

        if (field.type === 'textarea') {
          control = el('textarea', {
            class: 'textarea textarea-code',
            id: inputID,
            rows: '5',
            spellcheck: 'false',
            placeholder: field.placeholder || '',
            onInput: function () {
              store(field, control.value);
            },
          });
          control.value = typeof current === 'string' ? current : JSON.stringify(current, null, 2);
        } else if (field.type === 'checkbox') {
          control = el('input', {
            class: 'input',
            style: 'width:auto',
            type: 'checkbox',
            id: inputID,
            onChange: function () {
              store(field, control.checked);
            },
          });
          control.checked = current === true || current === 'true';
        } else {
          control = el('input', {
            class: 'input',
            type: field.type === 'number' ? 'number' : (field.type === 'password' ? 'password' : 'text'),
            id: inputID,
            autocomplete: 'off',
            spellcheck: 'false',
            placeholder: field.placeholder || '',
            value: String(current),
            onInput: function () {
              store(field, control.value);
            },
          });
        }

        function store(definition, value) {
          if (definition.type === 'checkbox') {
            draft[definition.key] = value;
          } else if (value === '' || value === null || value === undefined) {
            delete draft[definition.key];
          } else if (definition.type === 'number') {
            var parsed = Number(value);
            draft[definition.key] = isNaN(parsed) ? value : parsed;
          } else {
            draft[definition.key] = value;
          }
          clearFieldError(errorNode, control);
          scheduleValidation();
        }

        fieldRefs.push({ key: field.key, required: field.required === true, control: control, error: errorNode });

        var controlNode = field.type === 'checkbox'
          ? el('div', { class: 'checkbox-row' }, [
            control,
            el('label', { class: 'label', for: inputID, text: field.label || field.key }),
          ])
          : control;

        return el('div', { class: 'field' }, [
          field.type === 'checkbox'
            ? null
            : el('label', { class: 'label', for: inputID }, [
              document.createTextNode(field.label || field.key),
              field.required ? el('span', { class: 'req', 'aria-hidden': 'true', text: '*' }) : null,
            ]),
          controlNode,
          field.help ? el('p', { class: 'help', text: field.help }) : null,
          errorNode,
        ]);
      }

      function clearFieldError(errorNode, control) {
        errorNode.hidden = true;
        if (control) {
          control.removeAttribute('aria-invalid');
        }
      }

      function validateSuffix() {
        var value = suffixInput.value.trim();
        if (value === '') {
          clearFieldError(suffixError, suffixInput);
          return true;
        }
        var valid = /^[0-9a-fA-F:.]+\/[0-9]{1,3}$/.test(value);
        if (!valid) {
          suffixError.textContent = 'Expected an IPv6 prefix such as ::/64.';
          suffixError.hidden = false;
          suffixInput.setAttribute('aria-invalid', 'true');
          return false;
        }
        clearFieldError(suffixError, suffixInput);
        return true;
      }

      function validateJSONArea() {
        var value = jsonArea.value.trim();
        if (value === '') {
          jsonError.hidden = true;
          return true;
        }
        var parsed;
        try {
          parsed = JSON.parse(value);
        } catch (error) {
          jsonError.textContent = 'This is not valid JSON: ' + (error && error.message ? error.message : 'parse error');
          jsonError.hidden = false;
          return false;
        }
        if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
          jsonError.textContent = 'The provider options must be a JSON object.';
          jsonError.hidden = false;
          return false;
        }
        Object.keys(draft).forEach(function (key) {
          if (COMMON_KEYS.indexOf(key) === -1) {
            delete draft[key];
          }
        });
        Object.keys(parsed).forEach(function (key) {
          draft[key] = parsed[key];
        });
        jsonError.hidden = true;
        return true;
      }

      /* provider combobox --------------------------------------------- */

      function openProviderList() {
        var query = providerInput.value.trim().toLowerCase();
        filtered = providers.filter(function (provider) {
          if (query === '') {
            return true;
          }
          return String(provider.name).toLowerCase().indexOf(query) !== -1 ||
            String(provider.label || '').toLowerCase().indexOf(query) !== -1;
        });
        activeOption = -1;
        clear(providerList);
        if (filtered.length === 0) {
          providerList.appendChild(el('li', { class: 'combobox-empty', text: 'No provider matches.' }));
        } else {
          filtered.slice(0, 100).forEach(function (provider, position) {
            var option = el('li', {
              class: 'combobox-option',
              role: 'option',
              id: 'entry-provider-option-' + position,
              'aria-selected': 'false',
              onMousedown: function (event) {
                event.preventDefault();
                setProvider(provider.name);
              },
              onMouseenter: function () {
                setActiveOption(position);
              },
            }, [
              el('span', { text: provider.label || provider.name }),
              el('span', { class: 'mono', text: provider.name }),
            ]);
            providerList.appendChild(option);
          });
        }
        providerList.hidden = false;
        providerInput.setAttribute('aria-expanded', 'true');
        setCaretOpen(true);
      }

      function closeProviderList() {
        providerList.hidden = true;
        providerInput.setAttribute('aria-expanded', 'false');
        providerInput.removeAttribute('aria-activedescendant');
        activeOption = -1;
        setCaretOpen(false);
      }

      function setCaretOpen(open) {
        providerCaret.classList.toggle('is-open', open);
      }

      function setActiveOption(position) {
        var options = providerList.querySelectorAll('.combobox-option');
        if (options.length === 0) {
          return;
        }
        if (activeOption >= 0 && options[activeOption]) {
          options[activeOption].setAttribute('aria-selected', 'false');
        }
        activeOption = Math.max(0, Math.min(position, options.length - 1));
        options[activeOption].setAttribute('aria-selected', 'true');
        providerInput.setAttribute('aria-activedescendant', options[activeOption].id);
        options[activeOption].scrollIntoView({ block: 'nearest' });
      }

      function chooseActiveOption() {
        var options = providerList.querySelectorAll('.combobox-option');
        if (activeOption < 0 || !options[activeOption]) {
          return false;
        }
        var provider = filtered[activeOption];
        if (provider) {
          setProvider(provider.name);
        }
        return true;
      }

      function onProviderKey(event) {
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
          event.preventDefault();
          if (providerList.hidden) {
            openProviderList();
            setActiveOption(0);
            return;
          }
          setActiveOption(event.key === 'ArrowDown' ? activeOption + 1 : activeOption - 1);
          return;
        }
        if (event.key === 'Enter') {
          if (!providerList.hidden && activeOption >= 0) {
            event.preventDefault();
            chooseActiveOption();
          }
          return;
        }
        if (event.key === 'Escape') {
          if (!providerList.hidden) {
            event.preventDefault();
            event.stopPropagation();
            closeProviderList();
          }
          return;
        }
        if (event.key === 'Tab') {
          closeProviderList();
        }
      }

      /* live validation ---------------------------------------------- */

      function collect() {
        var result = {};
        Object.keys(draft).forEach(function (key) {
          result[key] = draft[key];
        });
        result.domain = domainInput.value.trim();
        var ipVersion = ipVersionSelect.value;
        if (isKnownIPVersion(ipVersion)) {
          result.ip_version = ipVersion;
        } else {
          delete result.ip_version;
        }
        var owner = ownerInput.value.trim();
        if (owner) {
          result.owner = owner;
        } else {
          delete result.owner;
        }
        var suffix = suffixInput.value.trim();
        if (suffix) {
          result.ipv6_suffix = suffix;
        } else {
          delete result.ipv6_suffix;
        }
        return result;
      }

      function showFormErrors(messages) {
        clear(formErrorBox);
        if (messages.length === 0) {
          formErrorBox.hidden = true;
          return;
        }
        formErrorBox.appendChild(icon(ICONS.error, {}));
        var body = el('div', { class: 'alert-body' }, [el('p', { class: 'alert-title', text: 'The entry was rejected' })]);
        messages.forEach(function (message) {
          body.appendChild(el('p', { style: 'margin-top:0.25rem', text: message }));
        });
        formErrorBox.appendChild(body);
        formErrorBox.hidden = false;
      }

      function showValidation(result) {
        clear(validationBox);
        if (result.valid) {
          validationBox.appendChild(el('div', { class: 'alert alert-success' }, [
            icon(ICONS.success, {}),
            el('div', { class: 'alert-body' }, [
              el('p', { text: 'This entry looks valid and can be saved.' }),
            ]),
          ]));
        } else {
          validationBox.appendChild(el('div', { class: 'alert alert-error' }, [
            icon(ICONS.error, {}),
            el('div', { class: 'alert-body' }, [
              el('p', { class: 'alert-title', text: 'This entry is not valid yet' }),
            ].concat((result.errors || []).map(function (message) {
              return el('p', { style: 'margin-top:0.25rem', text: message });
            }))),
          ]));
        }
        (result.warnings || []).forEach(function (warning) {
          validationBox.appendChild(el('div', { class: 'alert alert-warn', style: 'margin-top:0.5rem' }, [
            icon(ICONS.warn, {}),
            el('div', { class: 'alert-body' }, [el('p', { text: warning })]),
          ]));
        });
      }

      function clientValidate() {
        var messages = [];
        if (!draft.provider) {
          messages.push('Choose a provider.');
        }
        if (!domainInput.value.trim()) {
          messages.push('The domain is required.');
        }
        fieldRefs.forEach(function (ref) {
          if (!ref.required) {
            return;
          }
          var value = ref.control.type === 'checkbox' ? ref.control.checked : String(ref.control.value || '').trim();
          if (!value) {
            messages.push('The field ' + ref.key + ' is required.');
            ref.error.textContent = 'This field is required.';
            ref.error.hidden = false;
            ref.control.setAttribute('aria-invalid', 'true');
          } else {
            clearFieldError(ref.error, ref.control);
          }
        });
        if (!validateSuffix()) {
          messages.push('The IPv6 suffix is not a valid prefix.');
        }
        if (!validateJSONArea()) {
          messages.push('The provider options are not valid JSON.');
        }
        showFormErrors(messages);
        return messages;
      }

      function scheduleValidation() {
        if (validationTimer) {
          window.clearTimeout(validationTimer);
        }
        validationTimer = window.setTimeout(runValidation, 450);
      }

      function runValidation() {
        validationTimer = null;
        if (!draft.provider || !domainInput.value.trim()) {
          return;
        }
        var entry = collect();
        var seq = ++validationSeq;
        request('POST', apiURL('/settings/validate'), entry).then(function (result) {
          if (seq !== validationSeq || !result.ok || !result.data) {
            return;
          }
          showValidation({
            valid: result.data.valid === true,
            errors: Array.isArray(result.data.errors) ? result.data.errors : [],
            warnings: Array.isArray(result.data.warnings) ? result.data.warnings : [],
          });
        }).catch(function () {
          /* the live check is advisory, a failure must not block typing */
        });
      }

      /* assemble -------------------------------------------------- */

      var body = el('div', {}, [
        formErrorBox,
        el('div', { class: 'field' }, [
          el('label', { class: 'label', for: 'entry-provider' }, [
            document.createTextNode('Provider'),
            el('span', { class: 'req', 'aria-hidden': 'true', text: '*' }),
          ]),
          providerBox,
          el('p', { class: 'help', text: 'The DNS service holding the zone of the domain below.' }),
          providerLink,
        ]),
        el('div', { class: 'field' }, [
          el('label', { class: 'label', for: 'entry-domain' }, [
            document.createTextNode('Domain'),
            el('span', { class: 'req', 'aria-hidden': 'true', text: '*' }),
          ]),
          domainInput,
          el('p', { class: 'help', text: 'The full domain, or just the sub domain part: "*.example.com", ' +
            '"home.office" and "home,office" all resolve to example.com. The owner is derived from it.' }),
        ]),
        fieldsHost,
        el('div', { class: 'field' }, [
          el('label', { class: 'label', for: 'entry-ip-version', text: 'IP version' }),
          ipVersionSelect,
          el('p', { class: 'help', text: 'Which address families the record is updated with. Both by default.' }),
        ]),
        el('details', { class: 'advanced' }, [
          el('summary', { text: 'Advanced' }),
          advancedBody,
        ]),
        el('div', { style: 'margin-top:1rem' }, [validationBox]),
      ]);

      advancedBody.appendChild(el('div', { class: 'field' }, [
        el('label', { class: 'label', for: 'entry-owner', text: 'Owner' }),
        ownerInput,
        el('p', { class: 'help', text: 'Optional. Normally derived from the domain; when set, it takes ' +
          'precedence. Use "@" for the domain apex.' }),
      ]));
      advancedBody.appendChild(el('div', { class: 'field' }, [
        el('label', { class: 'label', for: 'entry-ipv6-suffix', text: 'IPv6 suffix' }),
        suffixInput,
        el('p', { class: 'help', text: 'Optional prefix used to derive the IPv6 address, for example ::/64.' }),
        suffixError,
      ]));

      var saveButton = el('button', { type: 'button', class: 'btn btn-primary' });
      saveButton.textContent = isEdit ? 'Save changes' : 'Add record';
      saveButton.addEventListener('click', submit);

      var footer = [];
      if (isEdit) {
        footer.push(el('button', {
          type: 'button',
          class: 'btn btn-danger',
          onClick: function () {
            handle.close();
            confirmDelete(index);
          },
        }, [icon(ICONS.trash, { class: 'alert-icon' }), document.createTextNode('Delete')]));
        footer.push(el('button', {
          type: 'button',
          class: 'btn',
          onClick: function () {
            handle.close();
            openEntryModal(null, draft);
          },
        }, [icon(ICONS.copy, { class: 'alert-icon' }), document.createTextNode('Duplicate')]));      }
      footer.push(el('span', { class: 'spacer' }));
      footer.push(el('button', {
        type: 'button',
        class: 'btn btn-ghost',
        text: 'Cancel',
        onClick: function () {
          handle.close();
        },
      }));
      footer.push(saveButton);

      var handle = openModal({
        title: isEdit ? 'Edit record #' + index : 'Add a record',
        subtitle: isEdit ? entryDomain(entry) : 'A domain, its owner and the credentials of your DNS provider.',
        body: body,
        footer: footer,
        onClose: function () {
          document.removeEventListener('mousedown', onDocumentPointerDown, true);
          document.removeEventListener('touchstart', onDocumentPointerDown, true);
          if (validationTimer) {
            window.clearTimeout(validationTimer);
            validationTimer = null;
          }
          validationSeq += 1;
        },
      });

      if (draft.provider) {
        /* Editing an existing entry, or duplicating one: preselect the
         * provider so its own fields are rendered straight away. */
        setProvider(draft.provider);
        if (isEdit) {
          scheduleValidation();
        }
      } else {
        renderProviderFields(null);
        providerInput.focus();
      }

      function submit() {
        if (clientValidate().length > 0) {
          return;
        }
        var entry = collect();
        if (Object.keys(entry).length === 0) {
          return;
        }
        showFormErrors([]);
        setBusy(saveButton, true, 'Saving');
        var path = isEdit
          ? apiURL('/settings/' + encodeURIComponent(String(index)))
          : apiURL('/settings');
        request(isEdit ? 'PUT' : 'POST', path, entry).then(function (result) {
          if (!result.ok) {
            showFormErrors(errorsOf(result));
            if (result.status === 422) {
              return;
            }
            toast('The entry could not be saved', 'error');
            return;
          }
          handle.close();
          adoptSettings(result.data);
          toast(isEdit ? 'Record updated' : 'Record added', 'success');
        }).catch(function () {
          showFormErrors(['the request could not be sent, check the updater is still running']);
        }).finally(function () {
          setBusy(saveButton, false);
        });
      }
    }

    /* --------------------------------------------------------- raw JSON */

    function syncRawFromSettings() {
      if (loadError) {
        nodes.rawTextarea.value = '';
        updateJSONState();
        return;
      }
      var document_ = { settings: settings.settings };
      nodes.rawTextarea.value = JSON.stringify(document_, null, 2);
      rawDirty = false;
      clear(nodes.rawErrors);
      nodes.rawStatus.textContent = '';
      updateJSONState();
    }

    function parseRaw() {
      var value = nodes.rawTextarea.value.trim();
      if (value === '') {
        return { ok: false, message: 'The document is empty.' };
      }
      var parsed;
      try {
        parsed = JSON.parse(value);
      } catch (error) {
        return { ok: false, message: 'Invalid JSON: ' + (error && error.message ? error.message : 'parse error') };
      }
      if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
        return { ok: false, message: 'The document must be a JSON object such as { "settings": [] }.' };
      }
      if (!Array.isArray(parsed.settings)) {
        return { ok: false, message: 'The "settings" key must be an array of entries.' };
      }
      parsed.settings.forEach(function (entry, position) {
        if (entry === null || typeof entry !== 'object' || Array.isArray(entry)) {
          throw { message: 'settings[' + position + '] must be a JSON object.' };
        }
      });
      return { ok: true, value: parsed };
    }

    function updateJSONState() {
      var parsed;
      try {
        parsed = parseRaw();
      } catch (error) {
        parsed = { ok: false, message: error && error.message ? error.message : 'invalid entry' };
      }
      if (parsed.ok) {
        nodes.rawSaveButton.removeAttribute('disabled');
        /* the validate button also stays disabled while a validation request
         * is in flight, so it cannot be fired twice */
        if (!busyState.has(nodes.rawValidateButton)) {
          nodes.rawValidateButton.removeAttribute('disabled');
        }
        nodes.rawStatus.textContent = 'Looks like valid JSON. ' +
          (rawDirty ? 'Unsaved changes.' : 'Matches the saved settings.');
        return;
      }
      nodes.rawValidateButton.setAttribute('disabled', '');
      nodes.rawSaveButton.setAttribute('disabled', '');
      nodes.rawStatus.textContent = 'Fix the JSON first: ' +
        (parsed.message || 'the document cannot be parsed');
    }

    function renderValidationErrors(result) {
      clear(nodes.rawErrors);
      var errors = Array.isArray(result.errors) ? result.errors : [];
      var warnings = Array.isArray(result.warnings) ? result.warnings : [];
      var list = el('div', { class: 'alert ' + (result.valid ? 'alert-success' : 'alert-error') }, [
        icon(result.valid ? ICONS.success : ICONS.error, {}),
        el('div', { class: 'alert-body' }, [
          el('p', { class: 'alert-title', text: result.valid
            ? 'The document is valid'
            : 'The document has ' + errors.length + ' problem(s)' }),
        ]),
      ]);
      var body = list.querySelector('.alert-body');
      errors.forEach(function (message) {
        body.appendChild(el('p', { style: 'margin-top:0.25rem', text: humaniseError(message) }));
      });
      warnings.forEach(function (message) {
        body.appendChild(el('p', { style: 'margin-top:0.25rem', text: message }));
      });
      nodes.rawErrors.appendChild(list);
    }

    function humaniseError(message) {
      var match = /^setting (\d+):\s*(.*)$/.exec(String(message));
      if (!match) {
        return String(message);
      }
      return 'Entry ' + (parseInt(match[1], 10) + 1) + ': ' + match[2];
    }

    function validateRaw() {
      var parsed;
      try {
        parsed = parseRaw();
      } catch (error) {
        clear(nodes.rawErrors);
        nodes.rawErrors.appendChild(el('div', { class: 'alert alert-error' }, [
          icon(ICONS.error, {}),
          el('div', { class: 'alert-body' }, [
            el('p', { text: error && error.message ? error.message : 'invalid entry' }),
          ]),
        ]));
        return;
      }
      if (!parsed.ok) {
        clear(nodes.rawErrors);
        nodes.rawErrors.appendChild(el('div', { class: 'alert alert-error' }, [
          icon(ICONS.error, {}),
          el('div', { class: 'alert-body' }, [el('p', { text: parsed.message })]),
        ]));
        return;
      }
      setBusy(nodes.rawValidateButton, true, 'Checking');
      request('POST', apiURL('/settings/validate'), parsed.value).then(function (result) {
        if (!result.ok) {
          clear(nodes.rawErrors);
          nodes.rawErrors.appendChild(el('div', { class: 'alert alert-error' }, [
            icon(ICONS.error, {}),
            el('div', { class: 'alert-body' }, [el('p', { text: errorsOf(result).join(' ') })]),
          ]));
          return;
        }
        renderValidationErrors(result.data || { valid: false, errors: [], warnings: [] });
      }).catch(function () {
        toast('The validation request could not be sent', 'error');
      }).finally(function () {
        setBusy(nodes.rawValidateButton, false);
      });
    }

    function saveRaw() {
      var parsed;
      try {
        parsed = parseRaw();
      } catch (error) {
        return;
      }
      if (!parsed.ok) {
        toast(parsed.message, 'error');
        return;
      }
      setBusy(nodes.rawSaveButton, true, 'Saving');
      request('PUT', apiURL('/settings'), parsed.value).then(function (result) {
        if (!result.ok) {
          clear(nodes.rawErrors);
          var box = el('div', { class: 'alert alert-error' }, [
            icon(ICONS.error, {}),
            el('div', { class: 'alert-body' }, [el('p', { class: 'alert-title', text: 'The document was rejected' })]),
          ]);
          var body = box.querySelector('.alert-body');
          errorsOf(result).forEach(function (message) {
            body.appendChild(el('p', { style: 'margin-top:0.25rem', text: humaniseError(message) }));
          });
          nodes.rawErrors.appendChild(box);
          toast('The document was not saved', 'error');
          return;
        }
        adoptSettings(result.data);
        toast('Settings saved and reloaded', 'success');
      }).catch(function () {
        toast('The save request could not be sent', 'error');
      }).finally(function () {
        setBusy(nodes.rawSaveButton, false);
      });
    }
  }

  /* ------------------------------------------------------------------ boot */

  /* keepStylesheetLast makes sure the offline baseline stays the last
   * stylesheet of the document: the Tailwind browser build injects its own
   * <style> element in <head> at runtime, which would otherwise come after
   * ours and win specificity ties. */
  function keepStylesheetLast() {
    var link = byID('ddns-styles');
    if (link && link.parentNode) {
      link.parentNode.appendChild(link);
    }
  }

  function boot() {
    keepStylesheetLast();
    initTheme();
    if (byID('app')) {
      initSettingsPage();
      return;
    }
    initRecordsPage();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
