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
