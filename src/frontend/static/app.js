const noteForm = document.getElementById('noteForm');
const titleInput = document.getElementById('titleInput');
const contentInput = document.getElementById('contentInput');
const statusMessage = document.getElementById('statusMessage');
const notesContainer = document.getElementById('notesContainer');
const noteTemplate = document.getElementById('noteTemplate');
const noteCount = document.getElementById('noteCount');
const exportButton = document.getElementById('exportButton');
const refreshButton = document.getElementById('refreshButton');

function setStatus(message) {
  statusMessage.textContent = message;
}

async function getNotes() {
  const response = await fetch('/api/notes');
  if (!response.ok) {
    throw new Error('Failed to load notes');
  }
  const payload = await response.json();
  return payload.notes || [];
}

async function createNote(title, content) {
  const response = await fetch('/api/notes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, content })
  });

  if (!response.ok) {
    throw new Error('Failed to save note');
  }
}

async function updateNote(id, title, content) {
  const response = await fetch(`/api/notes/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, content })
  });

  if (!response.ok) {
    throw new Error('Failed to update note');
  }
}

async function deleteNote(id) {
  const response = await fetch(`/api/notes/${id}`, {
    method: 'DELETE'
  });

  if (!response.ok) {
    throw new Error('Failed to delete note');
  }
}

function formatTimestamp(ts) {
  if (!ts) {
    return 'unknown time';
  }
  const date = new Date(ts);
  if (Number.isNaN(date.getTime())) {
    return ts;
  }
  return date.toLocaleString();
}

function renderEmptyState() {
  notesContainer.innerHTML = '';
  const paragraph = document.createElement('p');
  paragraph.className = 'empty-state';
  paragraph.textContent = 'No notes yet. Add your first workshop note.';
  notesContainer.appendChild(paragraph);
}

function updateCount(count) {
  noteCount.textContent = `${count} note${count === 1 ? '' : 's'}`;
}

function renderNotes(notes) {
  if (notes.length === 0) {
    updateCount(0);
    renderEmptyState();
    return;
  }

  updateCount(notes.length);
  notesContainer.innerHTML = '';

  for (const note of notes) {
    const fragment = noteTemplate.content.cloneNode(true);
    const card = fragment.querySelector('.note-card');
    const titleField = fragment.querySelector('.note-title');
    const contentField = fragment.querySelector('.note-content');
    const meta = fragment.querySelector('.note-meta');
    const saveButton = fragment.querySelector('.save-button');
    const deleteButton = fragment.querySelector('.delete-button');

    titleField.value = note.title;
    contentField.value = note.content;
    meta.textContent = `Created ${formatTimestamp(note.createdAt)} • Updated ${formatTimestamp(note.updatedAt)}`;

    saveButton.addEventListener('click', async () => {
      try {
        saveButton.disabled = true;
        await updateNote(note.id, titleField.value, contentField.value);
        setStatus(`Saved note #${note.id}`);
        await refreshNotes();
      } catch (error) {
        setStatus(error.message);
      } finally {
        saveButton.disabled = false;
      }
    });

    deleteButton.addEventListener('click', async () => {
      const confirmed = window.confirm(`Delete note #${note.id}?`);
      if (!confirmed) {
        return;
      }

      try {
        deleteButton.disabled = true;
        await deleteNote(note.id);
        setStatus(`Deleted note #${note.id}`);
        await refreshNotes();
      } catch (error) {
        setStatus(error.message);
      } finally {
        deleteButton.disabled = false;
      }
    });

    card.dataset.noteId = note.id;
    notesContainer.appendChild(fragment);
  }
}

async function refreshNotes() {
  try {
    const notes = await getNotes();
    renderNotes(notes);
  } catch (error) {
    setStatus(error.message);
  }
}

noteForm.addEventListener('submit', async event => {
  event.preventDefault();

  const title = titleInput.value.trim();
  const content = contentInput.value;

  try {
    await createNote(title, content);
    titleInput.value = '';
    contentInput.value = '';
    setStatus('Note saved');
    await refreshNotes();
  } catch (error) {
    setStatus(error.message);
  }
});

refreshButton.addEventListener('click', async () => {
  setStatus('Refreshing notes...');
  await refreshNotes();
  setStatus('Notes refreshed');
});

exportButton.addEventListener('click', async () => {
  try {
    const response = await fetch('/api/notes/export.md');
    if (!response.ok) {
      throw new Error('Failed to export notes');
    }

    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = 'workshop-notes.md';
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
    setStatus('Exported workshop-notes.md');
  } catch (error) {
    setStatus(error.message);
  }
});

refreshNotes();

// ---------------------------------------------------------------------------
// Tab switching – Notes ↔ Source Code
// ---------------------------------------------------------------------------

const tabNotes    = document.getElementById('tabNotes');
const tabSource   = document.getElementById('tabSource');
const panelNotes  = document.getElementById('panelNotes');
const panelSource = document.getElementById('panelSource');

let sourceTabInitialised = false;

function activateTab(tab) {
  const isSource = tab === tabSource;

  tabNotes.classList.toggle('tab-active', !isSource);
  tabNotes.setAttribute('aria-selected', String(!isSource));
  tabSource.classList.toggle('tab-active', isSource);
  tabSource.setAttribute('aria-selected', String(isSource));

  panelNotes.hidden  =  isSource;
  panelSource.hidden = !isSource;

  if (isSource && !sourceTabInitialised) {
    sourceTabInitialised = true;
    initSourceTab();
  }
}

tabNotes.addEventListener('click',  () => activateTab(tabNotes));
tabSource.addEventListener('click', () => activateTab(tabSource));

// ---------------------------------------------------------------------------
// Source Code tab – file tree + Monaco editor
// ---------------------------------------------------------------------------

let monacoEditor = null;

/**
 * Map a file path to a Monaco language identifier.
 */
function detectLanguage(path) {
  const base = path.split('/').pop();
  if (base === 'Containerfile' || base === 'Dockerfile') return 'dockerfile';
  const ext = base.slice(base.lastIndexOf('.') + 1).toLowerCase();
  const map = {
    go: 'go', mod: 'go', sum: 'plaintext',
    py: 'python',
    yaml: 'yaml', yml: 'yaml',
    js: 'javascript', ts: 'typescript',
    css: 'css', html: 'html',
    md: 'markdown', adoc: 'plaintext',
    json: 'json', sh: 'shell',
  };
  return map[ext] || 'plaintext';
}

/**
 * Build the file tree nav from a flat list of paths.
 * Root-level files appear first, then directories as collapsible groups.
 */
function buildFileTree(files, onSelect) {
  const fileTree = document.getElementById('fileTree');
  fileTree.innerHTML = '';

  // Group by top-level directory ('' = root-level files).
  const groups = new Map();
  for (const path of files) {
    const slash = path.indexOf('/');
    const dir   = slash === -1 ? '' : path.slice(0, slash);
    if (!groups.has(dir)) groups.set(dir, []);
    groups.get(dir).push(path);
  }

  // Root-level files first.
  for (const path of (groups.get('') || [])) {
    fileTree.appendChild(makeFileButton(path, path, onSelect));
  }

  // Then sorted directory groups.
  const dirs = [...groups.keys()].filter(d => d !== '').sort();
  for (const dir of dirs) {
    const details = document.createElement('details');
    details.open = true;

    const summary = document.createElement('summary');
    summary.className = 'tree-dir';
    summary.textContent = dir;
    details.appendChild(summary);

    for (const path of groups.get(dir)) {
      const label = path.slice(dir.length + 1); // strip "dir/" prefix
      details.appendChild(makeFileButton(label, path, onSelect));
    }

    fileTree.appendChild(details);
  }
}

function makeFileButton(label, path, onSelect) {
  const btn = document.createElement('button');
  btn.className = 'tree-file';
  btn.textContent = label;
  btn.dataset.path = path;
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tree-file').forEach(b => b.classList.remove('selected'));
    btn.classList.add('selected');
    onSelect(path);
  });
  return btn;
}

/**
 * Fetch a file from /api/code/<path> and display it in Monaco.
 */
async function loadSourceFile(path) {
  try {
    const response = await fetch(`/api/code/${path}`);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const text = await response.text();

    if (monacoEditor) {
      monaco.editor.setModelLanguage(monacoEditor.getModel(), detectLanguage(path));
      monacoEditor.setValue(text);
      monacoEditor.setScrollPosition({ scrollTop: 0 });
    }
  } catch (err) {
    if (monacoEditor) monacoEditor.setValue(`// Error loading ${path}: ${err.message}`);
  }
}

/**
 * Initialise Monaco, load the file listing, and wire up the tree.
 */
function initSourceTab() {
  require.config({
    paths: { vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.52.0/min/vs' },
  });

  require(['vs/editor/editor.main'], async () => {
    monacoEditor = monaco.editor.create(
      document.getElementById('monacoContainer'),
      {
        value: '// Select a file from the tree on the left.',
        language: 'plaintext',
        readOnly: true,
        theme: 'vs-dark',
        fontSize: 13,
        lineNumbers: 'on',
        minimap: { enabled: true },
        scrollBeyondLastLine: false,
        wordWrap: 'off',
        automaticLayout: true,
      }
    );

    try {
      const res = await fetch('/api/code');
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const { files = [] } = await res.json();

      if (files.length === 0) {
        monacoEditor.setValue(
          '// No embedded source files found.\n' +
          '// Source files are embedded at container build time.\n' +
          '// Run a container build to see the workshop source code here.'
        );
        return;
      }

      buildFileTree(files, loadSourceFile);

      // Auto-select and load the first file.
      const firstBtn = document.querySelector('.tree-file');
      if (firstBtn) {
        firstBtn.classList.add('selected');
        loadSourceFile(firstBtn.dataset.path);
      }
    } catch (err) {
      monacoEditor.setValue(`// Failed to load file list: ${err.message}`);
    }
  });
}

/**
 * openSourceFile(path) – switch to the Source Code tab and open a specific
 * file. Workshop AsciiDoc exercises can link to source code with:
 *
 *   <button onclick="openSourceFile('backend/main.go')">View source</button>
 *
 * or via a normal link:  href="javascript:openSourceFile('backend/main.go')"
 */
window.openSourceFile = function openSourceFile(path) {
  activateTab(tabSource);

  const tryLoad = () => {
    if (monacoEditor) {
      const btn = document.querySelector(`.tree-file[data-path="${CSS.escape(path)}"]`);
      if (btn) {
        document.querySelectorAll('.tree-file').forEach(b => b.classList.remove('selected'));
        btn.classList.add('selected');
      }
      loadSourceFile(path);
    } else {
      setTimeout(tryLoad, 100);
    }
  };
  tryLoad();
};
