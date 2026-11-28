(function () {
    function parseTags(raw) {
        if (!raw) return [];
        var parts = raw.split(',');
        var seen = {};
        var result = [];
        for (var i = 0; i < parts.length; i++) {
            var tag = parts[i].trim();
            if (!tag) continue;
            var key = tag.toLowerCase();
            if (seen[key]) continue;
            seen[key] = true;
            result.push(tag);
        }
        return result;
    }

            function postJSON(url, payload) {
            return fetch(url, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
        }).then(function (res) {
            if (!res.ok) throw new Error('Request failed');
            return res.json();
        });
}

    function syncAllSavedTags(savedTags) {
        var widgets = document.querySelectorAll('[data-status-tag-input]');
        widgets.forEach(function (w) {
            if (typeof w._renderSavedDropdown === 'function') {
                w._renderSavedDropdown(savedTags || []);
            }
        });
    }

        function initStatusTagWidget(container) {
            if (!container || container.dataset.tagWidgetInit === '1') return;
            container.dataset.tagWidgetInit = '1';

            var hiddenInput = container.querySelector('input[type="hidden"][name="status_tags"]');
            var chipsContainer = container.querySelector('[data-tag-chips]');
            var textInput = container.querySelector('[data-tag-text]');
            var toggleBtn = container.querySelector('[data-tag-dropdown-toggle]');
            var dropdown = container.querySelector('[data-saved-tags-dropdown]');
            if (!hiddenInput || !chipsContainer || !textInput || !dropdown) return;

            var tags = parseTags(hiddenInput.value);
            var savedTags = [];
            dropdown.querySelectorAll('[data-initial-saved-tag]').forEach(function (el) {
                var val = el.getAttribute('data-initial-saved-tag');
                if (val) savedTags.push(val);
            });

            var isOpen = false;

            function hasTag(list, tag) {
                var key = tag.toLowerCase();
                return list.some(function (t) { return t.toLowerCase() === key; });
            }

            function openDropdown() {
                isOpen = true;
                renderDropdown();
                dropdown.classList.remove('hidden');
            }

            function closeDropdown() {
                isOpen = false;
                dropdown.classList.add('hidden');
            }

            function addTag(raw) {
                var incoming = parseTags(raw);
                var changed = false;
                incoming.forEach(function (t) {
                    if (!hasTag(tags, t)) {
                        tags.push(t);
                        changed = true;
                    }
                    if (!hasTag(savedTags, t)) {
                        savedTags.push(t);
                        changed = true;
                    }
                });
                if (changed) render();
            }

            function removeTag(tag) {
                var key = tag.toLowerCase();
                tags = tags.filter(function (t) { return t.toLowerCase() !== key; });
                render();
            }

                function toggleTag(tag) {
                    if (hasTag(tags, tag)) {
                        removeTag(tag);
                    } else {
                        addTag(tag);
                    }
                }

                function renderDropdown(nextSaved) {
                    if (Array.isArray(nextSaved)) {
                        savedTags = nextSaved;
                    }
                    dropdown.innerHTML = '';

                        var filter = textInput.value.trim().toLowerCase();
                        var matches = savedTags.filter(function (t) {
                            return !filter || t.toLowerCase().indexOf(filter) !== -1;
                        });

                        if (filter && !hasTag(savedTags, textInput.value.trim())) {
                            var addRow = document.createElement('div');
                            addRow.className = 'status-tag-dropdown-item font-semibold text-emerald-700 dark:text-emerald-400';
                            addRow.textContent = '+ Add "' + textInput.value.trim() + '"';
                            addRow.addEventListener('mousedown', function (e) {
                                e.preventDefault();
                                addTag(textInput.value);
                                textInput.value = '';
                                renderDropdown();
                            });
                            dropdown.appendChild(addRow);
                        }

                        if (matches.length === 0 && !filter) {
                            var empty = document.createElement('div');
                            empty.className = 'px-2.5 py-2 text-[11px] text-slate-400 italic';
                            empty.textContent = 'No existing tags yet. Type above and press Enter.';
                            dropdown.appendChild(empty);
                            return;
                        }

                matches.forEach(function (tag) {
                    var selected = hasTag(tags, tag);
                    var row = document.createElement('div');
                    row.className = 'status-tag-dropdown-item' + (selected ? ' is-selected' : '');

                    var left = document.createElement('span');
                    left.className = 'flex items-center gap-1.5 flex-1 truncate';
                    left.textContent = (selected ? '✓ ' : '') + tag;
                    row.appendChild(left);

                        var delBtn = document.createElement('button');
                        delBtn.type = 'button';
                        delBtn.className = 'status-tag-dropdown-delete';
                        delBtn.setAttribute('aria-label', 'Delete saved tag ' + tag);
                        delBtn.title = 'Delete from existing tags';
                        delBtn.innerHTML = '&times;';
                        delBtn.addEventListener('mousedown', function (e) {
                            e.preventDefault();
                            e.stopPropagation();
                            postJSON('/students/status-tags/delete', { name: tag })
                                .then(function (data) {
                                    syncAllSavedTags(data.tags || []);
                                })
                                .catch(function () { });
                        });
                        row.appendChild(delBtn);

                            row.addEventListener('mousedown', function (e) {
                                e.preventDefault();
                                toggleTag(tag);
                                textInput.focus();
                            });

                            dropdown.appendChild(row);
                        });
                    }

                    container._renderSavedDropdown = renderDropdown;
                    container._setTags = function (csv) {
                        tags = parseTags(csv || '');
                        textInput.value = '';
                        render();
                    };

                function render() {
                    hiddenInput.value = tags.join(',');
                    chipsContainer.innerHTML = '';

                tags.forEach(function (tag) {
                    var chip = document.createElement('span');
                    chip.className = 'status-tag-chip';

                    var label = document.createElement('span');
                    label.textContent = tag;
                    chip.appendChild(label);

                    var removeBtn = document.createElement('button');
                    removeBtn.type = 'button';
                    removeBtn.className = 'status-tag-chip-remove';
                    removeBtn.setAttribute('aria-label', 'Remove ' + tag);
                    removeBtn.innerHTML = '&times;';
                    removeBtn.addEventListener('click', function () {
                        removeTag(tag);
                        textInput.focus();
                    });
                    chip.appendChild(removeBtn);

                    chipsContainer.appendChild(chip);
                });

            if (isOpen) renderDropdown();
        }

        textInput.addEventListener('focus', openDropdown);
        textInput.addEventListener('input', function () {
            if (!isOpen) openDropdown();
            else renderDropdown();
        });

        textInput.addEventListener('keydown', function (e) {
            if (e.key === 'Enter' || e.key === ',') {
                e.preventDefault();
                if (textInput.value.trim() !== '') {
                    addTag(textInput.value);
                    textInput.value = '';
                    if (isOpen) renderDropdown();
                }
            } else if (e.key === 'Backspace' && textInput.value === '' && tags.length > 0) {
                tags.pop();
                render();
            } else if (e.key === 'Escape') {
                closeDropdown();
            }
        });

    if (toggleBtn) {
        toggleBtn.addEventListener('mousedown', function (e) {
            e.preventDefault();
            if (isOpen) {
                closeDropdown();
            } else {
                openDropdown();
                textInput.focus();
            }
        });
    }

    document.addEventListener('click', function (e) {
        if (isOpen && !container.contains(e.target)) {
            closeDropdown();
        }
    });

    chipsContainer.addEventListener('click', function (e) {
        if (e.target === chipsContainer) {
            textInput.focus();
        }
    });

    render();
}

function initAllStatusTagWidgets(root) {
    var scope = root && root.querySelectorAll ? root : document;
    var widgets = scope.querySelectorAll('[data-status-tag-input]');
    widgets.forEach(initStatusTagWidget);
}

document.addEventListener('DOMContentLoaded', function () {
    initAllStatusTagWidgets(document);
});

document.addEventListener('htmx:load', function (e) {
    initAllStatusTagWidgets(e.detail && e.detail.elt ? e.detail.elt : document);
});
}) ();
