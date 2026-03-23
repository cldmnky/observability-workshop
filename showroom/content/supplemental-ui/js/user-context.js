// User Context Injection for Multi-User Workshops
// Fetches user-specific data and replaces placeholders in the page

(function() {
  'use strict';

  // Placeholders to replace (following Antora/showroom convention)
  const PLACEHOLDERS = {
    '{user}': 'user',
    '{openshift_console_url}': 'console_url',
    '{openshift_cluster_console_url}': 'console_url',
    '%openshift_cluster_console_url%': 'console_url',
    '{login_command}': 'login_command',
    '{openshift_cluster_ingress_domain}': 'openshift_cluster_ingress_domain',
    '{openshift_api_url}': 'api_url',
    '%perses_url%': 'perses_url',
    '{perses_url}': 'perses_url'
  };

  // Namespace literals used in workshop exercises that must be user-specific
  // Note: 'observability-demo' is NOT included because it's a shared COO namespace
  const EXERCISE_NAMESPACE_LITERALS = {
    'tracing-demo': (user) => `${user}-tracing-demo`
  };

  function escapeRegExp(text) {
    return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }

  function buildReplacementPairs(userData) {
    const pairs = [];

    // Placeholder replacements (marked with false for isLiteral)
    for (const [placeholder, key] of Object.entries(PLACEHOLDERS)) {
      if (userData[key]) {
        pairs.push([placeholder, userData[key], false]);
      }
    }

    // Namespace literal replacements (marked with true for isLiteral)
    if (userData.user) {
      for (const [literal, mapper] of Object.entries(EXERCISE_NAMESPACE_LITERALS)) {
        pairs.push([literal, mapper(userData.user), true]);
      }
    }

    return pairs;
  }

  // Fetch user info from the API endpoint
  async function fetchUserInfo() {
    try {
      const response = await fetch('/api/user-info', {
        credentials: 'include', // Include OAuth cookies
        headers: {
          'Accept': 'application/json'
        }
      });

      if (!response.ok) {
        console.warn('Failed to fetch user info:', response.status);
        return null;
      }

      return await response.json();
    } catch (error) {
      console.error('Error fetching user info:', error);
      return null;
    }
  }

  // Replace placeholders in text nodes
  function replaceInTextNode(node, replacementPairs) {
    let text = node.textContent;
    let replaced = false;

    for (const [findText, replaceText, isLiteral] of replacementPairs) {
      if (text.includes(findText)) {
        // For namespace literals, use negative lookbehind to avoid double replacement
        const pattern = isLiteral 
          ? new RegExp(`(?<!\\w-)${escapeRegExp(findText)}(?!-)`, 'g')
          : new RegExp(escapeRegExp(findText), 'g');
        text = text.replace(pattern, replaceText);
        replaced = true;
      }
    }

    if (replaced) {
      node.textContent = text;
    }
  }

  // Replace placeholders in attribute values (href, value, etc.)
  function replaceInAttributes(element, replacementPairs) {
    const attributes = ['href', 'value', 'data-url', 'content'];
    
    attributes.forEach(attr => {
      if (element.hasAttribute(attr)) {
        let value = element.getAttribute(attr);
        let replaced = false;

        for (const [findText, replaceText, isLiteral] of replacementPairs) {
          if (value.includes(findText)) {
            // For namespace literals, use negative lookbehind to avoid double replacement
            const pattern = isLiteral 
              ? new RegExp(`(?<!\\w-)${escapeRegExp(findText)}(?!-)`, 'g')
              : new RegExp(escapeRegExp(findText), 'g');
            value = value.replace(pattern, replaceText);
            replaced = true;
          }
        }

        if (replaced) {
          element.setAttribute(attr, value);
        }
      }
    });
  }

  // Walk the DOM tree and replace placeholders
  function replacePlaceholders(userData) {
    const replacementPairs = buildReplacementPairs(userData);

    const walker = document.createTreeWalker(
      document.body,
      NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT,
      null
    );

    let node;
    const elementsToProcess = [];

    while (node = walker.nextNode()) {
      if (node.nodeType === Node.TEXT_NODE) {
        // Process text nodes
        const text = node.textContent;
        if (text && replacementPairs.some(([findText]) => text.includes(findText))) {
          replaceInTextNode(node, replacementPairs);
        }
      } else if (node.nodeType === Node.ELEMENT_NODE) {
        // Collect elements with attributes to process
        elementsToProcess.push(node);
      }
    }

    // Process element attributes
    elementsToProcess.forEach(element => replaceInAttributes(element, replacementPairs));
  }

  // Show user info indicator in header
  function showUserIndicator(userData) {
    const navbar = document.querySelector('.navbar');
    if (!navbar) return;

    const userBadge = document.createElement('div');
    userBadge.className = 'user-badge';
    userBadge.style.cssText = `
      position: absolute;
      top: 10px;
      right: 20px;
      background: #0066cc;
      color: white;
      padding: 8px 16px;
      border-radius: 4px;
      font-size: 14px;
      font-weight: 600;
      box-shadow: 0 2px 4px rgba(0,0,0,0.2);
      z-index: 1000;
    `;
    userBadge.innerHTML = `
      <span style="opacity: 0.8;">Logged in as:</span>
      <span style="margin-left: 8px;">${userData.user}</span>
    `;

    navbar.appendChild(userBadge);
  }

  // Initialize user context injection
  async function init() {
    console.log('[User Context] Initializing...');
    
    const userData = await fetchUserInfo();
    
    if (!userData) {
      console.warn('[User Context] No user data available - using placeholder values');
      return;
    }

    console.log('[User Context] User data loaded for:', userData.user);
    
    // Replace placeholders in the DOM
    replacePlaceholders(userData);
    
    // Show user indicator
    showUserIndicator(userData);
    
    console.log('[User Context] Placeholders replaced successfully');
  }

  // Run when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();

// Copy button functionality for code blocks
(function () {
  'use strict';

  // Inject styles
  var style = document.createElement('style');
  style.textContent = [
    '.listingblock .content { position: relative; }',
    '.copy-btn {',
    '  position: absolute; top: 8px; right: 8px;',
    '  display: inline-flex; align-items: center;',
    '  padding: 3px 10px; font-size: 11px; font-family: inherit; font-weight: 500;',
    '  color: #444; background: #fff;',
    '  border: 1px solid #bbb; border-radius: 6px;',
    '  cursor: pointer; opacity: 0; line-height: 1.6; user-select: none;',
    '  letter-spacing: 0.01em;',
    '  transition: opacity 0.2s ease, background 0.2s ease, color 0.2s ease, border-color 0.2s ease;',
    '  z-index: 10;',
    '}',
    '.listingblock .content:hover .copy-btn { opacity: 1; }',
    '.copy-btn:hover { background: #e8e8e8; border-color: #888; color: #111; }',
    '.copy-btn:active { transform: scale(0.97); }',
    '.copy-btn.copied { color: #16a34a; border-color: #86efac; background: #f0fdf4; opacity: 1; }'
  ].join('\n');
  document.head.appendChild(style);

  function addCopyButtons() {
    document.querySelectorAll('.listingblock .content pre').forEach(function (pre) {
      if (pre.parentNode.querySelector('.copy-btn')) return;
      var btn = document.createElement('button');
      btn.className = 'copy-btn';
      btn.textContent = 'Copy';
      btn.setAttribute('aria-label', 'Copy code to clipboard');
      btn.addEventListener('click', function () {
        var code = pre.querySelector('code') || pre;
        var text = code.innerText || code.textContent;
        navigator.clipboard.writeText(text).then(function () {
          btn.textContent = 'Copied!';
          btn.classList.add('copied');
          setTimeout(function () { btn.textContent = 'Copy'; btn.classList.remove('copied'); }, 2000);
        }).catch(function () {
          var ta = document.createElement('textarea');
          ta.value = text;
          ta.style.cssText = 'position:fixed;top:0;left:0;opacity:0;';
          document.body.appendChild(ta);
          ta.select();
          try {
            document.execCommand('copy');
            btn.textContent = 'Copied!';
            btn.classList.add('copied');
            setTimeout(function () { btn.textContent = 'Copy'; btn.classList.remove('copied'); }, 2000);
          } catch (e) { btn.textContent = 'Error'; }
          document.body.removeChild(ta);
        });
      });
      pre.parentNode.style.position = 'relative';
      pre.parentNode.appendChild(btn);
    });
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', addCopyButtons);
  } else {
    addCopyButtons();
  }
  var observer = new MutationObserver(addCopyButtons);
  observer.observe(document.body, { childList: true, subtree: true });
})();
