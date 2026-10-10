// The Gx glue of the htmx adapter (REQ-ACT-09). htmx.js and idiomorph.js
// load before this file. The glue sets the htmx options that Gx needs, adds
// the swap styles of the Gx patch modes and applies the answers that the Gx
// runtime fetches itself: a form, live validation and a navigation.
(function () {
  'use strict'
  var htmx = window.htmx
  var Idiomorph = window.Idiomorph
  if (!htmx || !Idiomorph) return

  // A strict CSP has no unsafe-eval and no inline style (SI-11).
  htmx.config.allowEval = false
  htmx.config.includeIndicatorStyles = false
  // An answer with an error status holds patches too: the toast of a failed
  // action (REQ-ACT-10).
  htmx.config.responseHandling = [
    { code: '204', swap: false },
    { code: '.*', swap: true },
  ]

  // An answer of the adapter names itself. An error answer from a different
  // source, for example the 403 of the CSRF check or the error page of a
  // proxy, holds no patches: its body does not go into the element of the
  // request (REQ-ACT-10).
  document.addEventListener('htmx:beforeSwap', function (e) {
    var xhr = e.detail.xhr
    if (xhr && xhr.status >= 400 && xhr.getResponseHeader('Gx-Answer') !== 'patches') e.detail.shouldSwap = false
  })

  // A morph keeps the content that an island wrote (REQ-ISL-04).
  var callbacks = {
    beforeNodeMorphed: function (from) {
      return !(from.hasAttribute && from.hasAttribute('data-ignore-morph'))
    },
  }

  // The swap styles of the Gx patch modes (REQ-ACT-04). The content is the
  // children of the element that carries hx-swap-oob.
  htmx.defineExtension('gx', {
    handleSwap: function (style, target, fragment) {
      if (style === 'gx-morph') {
        return Idiomorph.morph(target, fragment.childNodes, { morphStyle: 'outerHTML', callbacks: callbacks })
      }
      if (style === 'gx-inner') {
        return Idiomorph.morph(target, fragment.childNodes, { morphStyle: 'innerHTML', callbacks: callbacks })
      }
      if (style === 'gx-replace') {
        var nodes = Array.prototype.slice.call(fragment.childNodes)
        target.replaceWith.apply(target, nodes)
        return nodes
      }
      return false
    },
  })
  var root = document.documentElement
  var ext = root.getAttribute('hx-ext')
  root.setAttribute('hx-ext', ext ? ext + ',gx' : 'gx')

  // A write of htmx carries the CSRF token, as a write of the Gx runtime
  // does (SI-03).
  document.addEventListener('htmx:configRequest', function (e) {
    var m = document.cookie.match(/(?:^|; )gx_csrf=([^;]*)/)
    if (m && e.detail.verb !== 'get') e.detail.headers['Gx-CSRF'] = decodeURIComponent(m[1])
  })

  // A write of htmx carries the hash of each fragment of the page, so the
  // server sends only the fragments that differ (REQ-ACT-15, REQ-ACT-16). A
  // page with too many fragments for one header sends none.
  document.addEventListener('htmx:configRequest', function (e) {
    if (e.detail.verb === 'get') return
    var parts = []
    var size = 0
    var els = document.querySelectorAll('[data-gx-h][id]')
    for (var i = 0; i < els.length; i++) {
      var part = els[i].id + '=' + (els[i].getAttribute('data-gx-h') || '')
      size += part.length + 1
      if (size > 4096) return
      parts.push(part)
    }
    if (parts.length > 0) e.detail.headers['Gx-Fragments'] = parts.join(',')
  })

  // The Gx runtime calls apply with the answer of a request that it made.
  window.__gxAdapter = {
    apply: function (res) {
      var to = res.headers.get('HX-Redirect')
      if (to) {
        location.href = to
        return Promise.resolve()
      }
      var transition = (res.headers.get('HX-Reswap') || '').indexOf('transition:true') >= 0
      return res.text().then(function (html) {
        var parsed = document.createElement('template')
        parsed.innerHTML = html
        var head = parsed.content.querySelector('meta[data-gx-head]')
        if (head && window.__gx && window.__gx.mergeHead) window.__gx.mergeHead(head.getAttribute('content') || '')
        return new Promise(function (resolve) {
          htmx.swap(
            document.body,
            html,
            { swapStyle: 'none', swapDelay: 0, settleDelay: 0, transition: transition },
            { afterSettleCallback: resolve },
          )
        })
      })
    },
  }
})()
