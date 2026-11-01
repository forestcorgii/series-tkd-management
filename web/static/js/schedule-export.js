/**
 * schedule-export.js
 * Series Taekwondo Management System (STMS)
 * Modular vanilla JavaScript engine for client-side DOM-to-image schedule snapshot export.
 * Version: 2.6.0 - Dynamic System Theme Detection (Light/Dark), Clean Card Typography, Zero Attendance
 */

(function (root, factory) {
    if (typeof define === 'function' && define.amd) {
        define([], factory);
    } else if (typeof module === 'object' && module.exports) {
        module.exports = factory();
    } else {
        root.STMSExport = factory();
        root.exportScheduleSnapshot = root.STMSExport.exportScheduleSnapshot;
        root.to12Hour = root.STMSExport.to12Hour;
        root.formatTimeRange12H = root.STMSExport.formatTimeRange12H;
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    const HTML2CANVAS_CDN = 'https://cdnjs.cloudflare.com/ajax/libs/html2canvas/1.4.1/html2canvas.min.js';

    /**
     * Escape special HTML characters to prevent XSS in cloned DOM
     */
    function escapeHtml(str) {
        if (!str) return '';
        return String(str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    }

    /**
     * Converts a 24-hour time string ("17:00", "09:30", "9:30") to 12-hour AM/PM ("5:00 PM", "9:30 AM")
     * @param {string} t
     * @returns {string}
     */
    function to12Hour(t) {
        if (!t) return '';
        t = t.trim();
        const m = t.match(/^(\d{1,2}):(\d{2})$/);
        if (!m) return t;
        let hour = parseInt(m[1], 10);
        const min = m[2];
        const ampm = hour >= 12 ? 'PM' : 'AM';
        hour = hour % 12;
        if (hour === 0) hour = 12;
        return `${hour}:${min} ${ampm}`;
    }

    /**
     * Converts a time range string like "10:00 - 12:00" or "17:00 – 18:15" to "10:00 AM - 12:00 PM"
     * @param {string} rangeStr
     * @returns {string}
     */
    function formatTimeRange12H(rangeStr) {
        if (!rangeStr) return '';
        return rangeStr.replace(/(\d{1,2}:\d{2})\s*[-–—]\s*(\d{1,2}:\d{2})/g, function (_, t1, t2) {
            return to12Hour(t1) + ' - ' + to12Hour(t2);
        });
    }

    /**
     * Dynamically loads html2canvas (local asset first, fallback to CDN).
     * @returns {Promise<Function>}
     */
    function loadHtml2Canvas() {
        if (typeof window.html2canvas === 'function') {
            return Promise.resolve(window.html2canvas);
        }

        return new Promise((resolve, reject) => {
            const existingScript = document.querySelector('script[src*="html2canvas"]');
            if (existingScript) {
                if (typeof window.html2canvas === 'function') {
                    return resolve(window.html2canvas);
                }
                existingScript.addEventListener('load', () => resolve(window.html2canvas));
                existingScript.addEventListener('error', () => {
                    const fallbackScript = document.createElement('script');
                    fallbackScript.src = '/static/js/html2canvas.min.js';
                    fallbackScript.onload = () => {
                        if (typeof window.html2canvas === 'function') resolve(window.html2canvas);
                        else reject(new Error('Local html2canvas loaded but window.html2canvas unavailable'));
                    };
                    fallbackScript.onerror = () => reject(new Error('Failed to load html2canvas'));
                    document.head.appendChild(fallbackScript);
                });
                return;
            }

            // Load local asset directly
            const script = document.createElement('script');
            script.src = '/static/js/html2canvas.min.js';
            script.onload = () => {
                if (typeof window.html2canvas === 'function') resolve(window.html2canvas);
                else reject(new Error('html2canvas loaded but window.html2canvas is not available'));
            };
            script.onerror = () => {
                const cdnScript = document.createElement('script');
                cdnScript.src = HTML2CANVAS_CDN;
                cdnScript.crossOrigin = 'anonymous';
                cdnScript.referrerPolicy = 'no-referrer';
                cdnScript.onload = () => {
                    if (typeof window.html2canvas === 'function') resolve(window.html2canvas);
                    else reject(new Error('CDN html2canvas loaded but window.html2canvas unavailable'));
                };
                cdnScript.onerror = () => reject(new Error('Failed to load html2canvas from both local and CDN'));
                document.head.appendChild(cdnScript);
            };
            document.head.appendChild(script);
        });
    }

    /**
     * Resolves schedule metadata (anchor date, date range label, venue name)
     */
    function getScheduleMetadata() {
        const wrapper = document.getElementById('session-cards-wrapper');
        let anchorDate = '';
        let dateRangeTitle = '';

        if (wrapper) {
            anchorDate = wrapper.getAttribute('data-anchor-date') || wrapper.getAttribute('data-selected-date') || '';
            dateRangeTitle = wrapper.getAttribute('data-calendar-title') || '';
        }

        if (!anchorDate || !/^\d{4}-\d{2}-\d{2}$/.test(anchorDate)) {
            const now = new Date();
            const y = now.getFullYear();
            const m = String(now.getMonth() + 1).padStart(2, '0');
            const d = String(now.getDate()).padStart(2, '0');
            anchorDate = `${y}-${m}-${d}`;
        }

        let venueName = 'All Branches / Venues';
        const locSelect = document.querySelector('select[name="location_id"]');
        if (locSelect && locSelect.value && locSelect.selectedOptions && locSelect.selectedOptions.length > 0) {
            const txt = locSelect.selectedOptions[0].textContent.trim();
            if (txt && txt.toLowerCase() !== 'all locations') {
                venueName = txt;
            }
        }

        return {
            anchorDate,
            dateRangeTitle: dateRangeTitle || `Schedule for ${anchorDate}`,
            venueName
        };
    }

    /**
     * Prepares the cloned DOM element before rendering to canvas.
     * Follows the active system theme (Dark or Light), converts times to 12-hour AM/PM,
     * removes Coach, Admin, Staff, internal notes, and attendant count,
     * handles overlapping split-column cards cleanly, and formats for 1080px social posts.
     */
        function prepareCloneForSocialExport(clonedDoc, clonedElement, metadata, isDark) {
            // 1. Sync theme class on cloned document
            if (clonedDoc.documentElement) {
                if (isDark) {
                    clonedDoc.documentElement.classList.add('dark');
                    clonedDoc.documentElement.classList.remove('light');
                } else {
                    clonedDoc.documentElement.classList.remove('dark');
                    clonedDoc.documentElement.classList.add('light');
                }
            }

            // 2. Set export canvas dimensions and theme styling
            const bgColor = isDark ? '#111827' : '#FFFFFF';
            const textColor = isDark ? '#F9FAFB' : '#0F172A';
            const borderColor = isDark ? '#374151' : '#E2E8F0';
            const subtextColor = isDark ? '#9CA3AF' : '#64748B';
            const brandRed = isDark ? '#DC2626' : '#990303';
            const cardBg = isDark ? '#1F2937' : '#FFFFFF';
            const cardBorder = isDark ? '#374151' : '#CBD5E1';

            clonedElement.style.width = '1080px';
            clonedElement.style.maxWidth = '1080px';
            clonedElement.style.minWidth = '1080px';
            clonedElement.style.backgroundColor = bgColor;
            clonedElement.style.color = textColor;
            clonedElement.style.padding = '24px 20px 28px 20px';
            clonedElement.style.borderRadius = '0px';
            clonedElement.style.boxSizing = 'border-box';
            clonedElement.style.boxShadow = 'none';
            clonedElement.style.border = 'none';
            clonedElement.style.fontFamily = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Montserrat', sans-serif";

            // 3. Remove header toolbars, filter dropdowns, and buttons
            const elementsToRemove = [
                '#filter-toggle-btn',
                '#export-schedule-btn',
                '#session-filter-container',
                '#session-filter-form',
                '.tooltip-icon',
                '.tooltip-content',
                '.has-tooltip',
                '.searchable-select-container'
            ];
            elementsToRemove.forEach(sel => {
                clonedElement.querySelectorAll(sel).forEach(el => el.remove());
            });

            // Hide original page header within the panel if present
            const panelHeaders = clonedElement.querySelectorAll('.border-b');
            panelHeaders.forEach(ph => {
                if (ph.textContent.includes('Dojang Class Calendar') || ph.parentElement === clonedElement) {
                    ph.remove();
                }
            });

                // 4. Remove the Week Navigation Bar
                clonedElement.querySelectorAll('#calendar-week-nav-bar').forEach(el => el.remove());
                clonedElement.querySelectorAll('div').forEach(el => {
                    if (el.className && typeof el.className === 'string' &&
                        (el.className.includes('bg-slate-50/80') || el.className.includes('dark:bg-slate-900/60'))) {
                        if (el.querySelector('button') || el.textContent.includes('Classes')) {
                            el.remove();
                        }
                    }
                });

                    // 5. Expand calendar scroll container to full 1080px view (no scrollcuts or cutoffs)
                    const scrollContainer = clonedElement.querySelector('#calendar-scroll-container');
                    if (scrollContainer) {
                        scrollContainer.style.overflow = 'visible';
                        scrollContainer.style.width = '100%';
                    }

                    // Expand scrollable timeline body
                    const scrollBody = clonedElement.querySelector('.max-h-\\[720px\\]') ||
                        clonedElement.querySelector('[class*="max-h-"]');
                    if (scrollBody) {
                        scrollBody.style.maxHeight = 'none';
                        scrollBody.style.overflow = 'visible';
                    }

                    // Ensure all 7 day columns are visible on desktop/mobile clones
                    clonedElement.querySelectorAll('.hidden.md\\:block, [class*="hidden md:block"]').forEach(el => {
                        el.classList.remove('hidden');
                        el.style.display = 'block';
                    });

                    // Force 7-column layout on day headers and timeline grid
                    clonedElement.querySelectorAll('[class*="grid-cols-[48px"]').forEach(grid => {
                        grid.style.display = 'grid';
                        grid.style.gridTemplateColumns = '64px repeat(7, 1fr)';
                    });

                    // Style the outer calendar grid box
                    clonedElement.querySelectorAll('.rounded-xl.border, .border-slate-200\\/80, .border-slate-800').forEach(el => {
                        el.style.borderColor = borderColor;
                        el.style.backgroundColor = isDark ? '#111827' : '#FFFFFF';
                    });

                    // Style Day Column Headers
                    const dayHeaderRow = clonedElement.querySelector('.grid.sticky.top-0, [class*="sticky top-0"]');
                    if (dayHeaderRow) {
                        dayHeaderRow.style.backgroundColor = isDark ? '#1F2937' : '#F8FAFC';
                        dayHeaderRow.style.borderBottom = `1px solid ${borderColor}`;
                        dayHeaderRow.querySelectorAll('div').forEach(d => {
                            d.style.borderColor = borderColor;
                        });
                        dayHeaderRow.querySelectorAll('span').forEach(sp => {
                            if (!sp.classList.contains('bg-[#990303]') && !sp.classList.contains('bg-red-600') && !sp.style.backgroundColor) {
                                if (sp.textContent.match(/^[A-Z]{3}$/)) {
                                    sp.style.color = isDark ? '#9CA3AF' : '#475569';
                                    sp.style.fontWeight = '800';
                                } else if (sp.textContent.match(/^\d{1,2}$/)) {
                                    sp.style.color = textColor;
                                    sp.style.fontWeight = '800';
                                } else {
                                    sp.style.color = subtextColor;
                                }
                            }
                        });
                    }

                    // Style Left Time Gutter and Grid Lines
                    clonedElement.querySelectorAll('.border-r').forEach(el => {
                        el.style.borderColor = borderColor;
                    });
                    clonedElement.querySelectorAll('.border-b').forEach(el => {
                        el.style.borderColor = isDark ? 'rgba(55, 65, 81, 0.4)' : '#F1F5F9';
                    });

                    // 6. Clean up interactive dropzone slots & "click to add" text
                    clonedElement.querySelectorAll('.calendar-slot-dropzone').forEach(dz => {
                        dz.style.pointerEvents = 'none';
                        dz.style.cursor = 'default';
                        dz.style.borderColor = isDark ? 'rgba(55, 65, 81, 0.4)' : '#F1F5F9';
                        dz.querySelectorAll('span').forEach(sp => {
                            sp.style.display = 'none';
                        });
                    });

                    // 7. Remove the "Now" red time line & gutter pill completely
                    clonedElement.querySelectorAll('#calendar-now-pill, #calendar-now-line, .calendar-today-header .w-3.h-3.bg-red-600').forEach(el => el.remove());
                    clonedElement.querySelectorAll('.bg-red-600').forEach(el => {
                        const text = el.textContent.trim();
                        if (text.includes('AM') || text.includes('PM') || el.classList.contains('rounded-full')) {
                            const parent = el.closest('div');
                            if (parent && parent !== clonedElement) {
                                parent.remove();
                            } else {
                                el.remove();
                            }
                        }
                    });

                    // 8. Clean up interactive links and buttons
                    clonedElement.querySelectorAll('.calendar-session-card a, .calendar-session-card button').forEach(el => el.remove());
                    clonedElement.querySelectorAll('a[href*="/login"], a[href*="redirect="]').forEach(el => el.remove());

                    // 9. Process each calendar card:
                    // - Theme-aware card styling
                    // - Convert time format to 12-hour AM/PM with compact split handling
                    // - Remove Coach, Admin, Staff
                    // - Remove Internal Notes
                    // - Remove all attendance info (no icon, number, or text)
                    // - Prevent vertical text clipping
                    clonedElement.querySelectorAll('.calendar-session-card').forEach(card => {
                        // Remove overflow-hidden on the card and parent so text is never vertically clipped
                        card.classList.remove('overflow-hidden');
                        card.style.overflow = 'visible';
                        card.style.backgroundColor = cardBg;
                        card.style.borderColor = cardBorder;
                        card.style.borderWidth = '1px';
                        card.style.borderStyle = 'solid';

                        // Distinct left category indicator border
                        const catColor = card.getAttribute('data-color') || brandRed;
                        card.style.borderLeft = `4px solid ${catColor}`;

                        card.style.color = textColor;
                        card.style.boxShadow = isDark ? '0 2px 5px rgba(0, 0, 0, 0.4)' : '0 2px 5px rgba(0, 0, 0, 0.05)';
                        card.style.padding = '6px 7px';
                        card.style.display = 'flex';
                        card.style.flexDirection = 'column';
                        card.style.justifyContent = 'flex-start';
                        card.style.gap = '3px';
                        card.style.boxSizing = 'border-box';

                        if (card.parentElement) {
                            card.parentElement.style.overflow = 'visible';
                        }

                            // Detect if card is in a split overlapping column (narrow width)
                            const cardWidthStr = card.style.width || '';
                            const isSplitCard = !cardWidthStr.includes('100%') && (cardWidthStr.includes('%') || (parseFloat(card.style.width) > 0 && parseFloat(card.style.width) < 95));

                            // Ensure short-duration cards have minimum comfortable height
                            const currentHeight = parseFloat(card.style.height) || 0;
                            if (currentHeight < 56) {
                                card.style.minHeight = '56px';
                                card.style.zIndex = '12';
                            }

                            // Remove Coach, Admin, and Staff
                            card.querySelectorAll('p').forEach(p => {
                                const txt = p.textContent.toLowerCase();
                                if (txt.includes('coach') || txt.includes('admin') || txt.includes('staff') || txt.includes('unassigned')) {
                                    p.remove();
                                }
                            });

                            // Remove Internal Curriculum / Staff Notes
                            card.querySelectorAll('p').forEach(p => {
                                const txt = p.textContent.trim();
                                const notesAttr = card.getAttribute('data-notes') || '';
                                if (p.classList.contains('italic') || (txt.startsWith('"') && txt.endsWith('"')) || (notesAttr && txt.includes(notesAttr))) {
                                    p.remove();
                                }
                            });

                            // Remove all attendance info completely (no icon, count, or text)
                            card.querySelectorAll('.border-t, [class*="border-t"]').forEach(el => el.remove());
                            card.querySelectorAll('span, p, div').forEach(el => {
                                const txt = el.textContent.trim();
                                if (txt.includes('👥') || txt.toLowerCase().includes('attending') || txt.toLowerCase().includes('attendant')) {
                                    el.remove();
                                }
                            });

                            // Resolve 12-hour AM/PM time strings
                            const startTimeRaw = card.getAttribute('data-start') || '';
                            const endTimeRaw = card.getAttribute('data-end') || '';
                            const t1 = startTimeRaw ? to12Hour(startTimeRaw) : '';
                            const t2 = endTimeRaw ? to12Hour(endTimeRaw) : '';

                            // Update <h4> time/title text with compact handling for narrow split cards
                            const titleEl = card.querySelector('h4');
                            if (titleEl) {
                                titleEl.classList.remove('truncate', 'leading-tight');
                                titleEl.style.overflow = 'visible';
                                titleEl.style.textOverflow = 'clip';
                                titleEl.style.whiteSpace = 'normal';
                                titleEl.style.lineHeight = '1.3';
                                titleEl.style.fontFamily = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Montserrat', sans-serif";
                                titleEl.style.color = textColor;
                                titleEl.style.marginTop = '2px';
                                titleEl.style.marginBottom = '2px';

                                const rawText = titleEl.textContent.trim();
                                const hasTimePattern = /\d{1,2}:\d{2}/.test(rawText);

                                    if (hasTimePattern) {
                                        if (isSplitCard && t1 && t2) {
                                            titleEl.style.fontSize = '8.5px';
                                            titleEl.style.fontWeight = '800';
                                            titleEl.style.whiteSpace = 'nowrap';
                                            titleEl.innerHTML = `<span style="display:block;line-height:1.15;font-size:8.5px;font-weight:800;white-space:nowrap;">${t1}</span><span style="display:block;line-height:1.15;font-size:8px;color:${subtextColor};font-weight:700;white-space:nowrap;">${t2}</span>`;
                                        } else {
                                            titleEl.style.fontSize = '10.5px';
                                            titleEl.style.fontWeight = '800';
                                            titleEl.textContent = (t1 && t2) ? `${t1} - ${t2}` : formatTimeRange12H(rawText);
                                        }
                                    } else {
                                        // Regular text title (e.g., TSET, STANDBY, etc.)
                                        titleEl.style.fontSize = isSplitCard ? '9.5px' : '11px';
                                        titleEl.style.fontWeight = '800';
                                    }
                                }

                            // Update secondary time paragraphs (used in duty/event cards)
                            card.querySelectorAll('p.font-mono, p').forEach(p => {
                                const txt = p.textContent.trim();
                                if (/\d{1,2}:\d{2}\s*[-–—]\s*\d{1,2}:\d{2}/.test(txt)) {
                                    p.classList.remove('truncate');
                                    p.style.overflow = 'visible';
                                    p.style.lineHeight = '1.3';
                                    p.style.fontFamily = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, monospace";

                                    if (isSplitCard && t1 && t2) {
                                        p.style.fontSize = '8.5px';
                                        p.style.fontWeight = '700';
                                        p.style.color = subtextColor;
                                        p.innerHTML = `<span style="display:block;line-height:1.2;">${t1}</span><span style="display:block;line-height:1.2;font-size:8px;">${t2}</span>`;
                                    } else {
                                        p.style.fontSize = '9.5px';
                                        p.style.fontWeight = '700';
                                        p.style.color = isDark ? '#E5E7EB' : '#334155';
                                        p.textContent = (t1 && t2) ? `${t1} - ${t2}` : formatTimeRange12H(txt);
                                    }
                                }
                            });

                            // Polish Location paragraph
                            card.querySelectorAll('p').forEach(p => {
                                if (p.textContent.includes('📍')) {
                                    p.classList.remove('truncate');
                                    p.style.overflow = 'visible';
                                    p.style.lineHeight = '1.25';
                                    p.style.fontSize = isSplitCard ? '8px' : '9px';
                                    p.style.color = subtextColor;
                                    p.style.fontWeight = '600';
                                    p.style.marginTop = '1px';
                                    p.style.fontFamily = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif";
                                }
                            });

                            // Polish status & type badges
                            card.querySelectorAll('.flex.items-center.justify-between span').forEach(badge => {
                                const badgeText = badge.textContent.trim();
                                if (badgeText === 'CLOSED' || badgeText === 'DONE') {
                                    badge.style.backgroundColor = isDark ? '#374151' : '#F1F5F9';
                                    badge.style.color = isDark ? '#D1D5DB' : '#475569';
                                    badge.style.border = `1px solid ${isDark ? '#4B5563' : '#CBD5E1'}`;
                                } else if (badgeText === 'LIVE') {
                                    badge.style.backgroundColor = '#10B981';
                                    badge.style.color = '#FFFFFF';
                                }
                                badge.style.fontFamily = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif";
                                badge.style.fontSize = isSplitCard ? '7.5px' : '8.5px';
                                badge.style.fontWeight = '700';
                            });
                        });

                        // 10. Remove the administrative roster table at the bottom ("Class Rosters & Quick Operations")
                        clonedElement.querySelectorAll('#calendar-roster-section, #classes-list, #session-table, .space-y-3.pt-2').forEach(el => el.remove());
                        clonedElement.querySelectorAll('table').forEach(tbl => {
                            const wrapper = tbl.closest('.space-y-3') || tbl.parentElement;
                            if (wrapper && wrapper !== clonedElement) {
                                wrapper.remove();
                            } else {
                                tbl.remove();
                            }
                        });

                        // 11. Prepend Branded Social Header (Theme-Aware with Valid HTML)
                        const headerContainer = document.createElement('div');
                        headerContainer.className = 'stms-export-social-header';
                        headerContainer.innerHTML = `
                        <div style="display: flex; justify-content: space-between; align-items: center; border-bottom: 2px solid ${brandRed}; padding-bottom: 14px; margin-bottom: 16px;">
                            <div style="display: flex; align-items: center; gap: 14px;">
                                <div style="width: 46px; height: 46px; border-radius: 10px; background: linear-gradient(135deg, ${brandRed}, #7D0202); display: flex; align-items: center; justify-content: center; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Outfit', sans-serif; font-weight: 900; color: #FFFFFF; font-size: 22px; box-shadow: 0 4px 12px ${isDark ? 'rgba(220, 38, 38, 0.35)' : 'rgba(153, 3, 3, 0.25)'}; flex-shrink: 0;">
                                    ST
                                </div>
                                <div>
                                    <div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Outfit', sans-serif; font-size: 22px; font-weight: 900; letter-spacing: 0.04em; color: ${textColor}; text-transform: uppercase; line-height: 1.1;">
                                        SERIES TAEKWONDO
                                    </div>
                                    <div style="font-size: 11px; font-weight: 700; color: ${brandRed}; text-transform: uppercase; letter-spacing: 0.08em; display: flex; align-items: center; gap: 6px; margin-top: 3px;">
                                        <span>📍 ${escapeHtml(metadata.venueName)}</span>
                                        <span>•</span>
                                        <span>Class Schedule</span>
                                    </div>
                                </div>
                            </div>
                            <div style="text-align: right;">
                                <div style="font-size: 10px; font-weight: 800; text-transform: uppercase; letter-spacing: 0.1em; color: ${subtextColor};">
                                    OFFICIAL SCHEDULE
                                </div>
                                <div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Outfit', sans-serif; font-size: 16px; font-weight: 800; color: ${textColor}; margin-top: 2px;">
                                    ${escapeHtml(metadata.dateRangeTitle)}
                                </div>
                            </div>
                        </div>
                        `;
        clonedElement.prepend(headerContainer);

        // 12. Append Branded Social Footer (Theme-Aware)
        const footerContainer = document.createElement('div');
        footerContainer.className = 'stms-export-social-footer';
        footerContainer.innerHTML = `
            <div style="display: flex; justify-content: space-between; align-items: center; border-top: 1px solid ${borderColor}; padding-top: 14px; margin-top: 16px; font-size: 11px; color: ${subtextColor};">
                <div style="display: flex; align-items: center; gap: 8px;">
                    <span style="display: inline-block; width: 8px; height: 8px; border-radius: 50%; background-color: ${brandRed};"></span>
                    <span style="font-weight: 700; color: ${isDark ? '#E5E7EB' : '#1E293B'};">Series Taekwondo Management System</span>
                </div>
                <div style="font-weight: 600; color: ${subtextColor};">
                    🥋 Follow <strong style="color: ${textColor};">@seriestkd</strong> &bull; Inquire &amp; Reserve: <strong style="color: ${textColor};">seriestkd.com</strong>
                </div>
            </div>
        `;
        clonedElement.appendChild(footerContainer);
    }

    /**
     * Main export function
     * @param {HTMLElement} [buttonEl] - Triggering button element for feedback state
     */
    async function exportScheduleSnapshot(buttonEl) {
        const btn = buttonEl || document.getElementById('export-schedule-btn');
        let originalContent = '';
        let originalDisabled = false;

        // Visual feedback state
        if (btn) {
            originalContent = btn.innerHTML;
            originalDisabled = btn.disabled;
            btn.disabled = true;
            btn.setAttribute('aria-busy', 'true');
            btn.innerHTML = `
                            < svg class="w-3.5 h-3.5 animate-spin text-rose-500 inline-block mr-1" fill = "none" viewBox = "0 0 24 24" >
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
                </svg >
                            <span class="btn-text">⏳ Generating...</span>
                        `;
        }

        try {
            // 1. Locate schedule container
            const scheduleContainer = document.getElementById('schedule') ||
                                     document.getElementById('classes-container') ||
                                     document.getElementById('session-cards-wrapper');

            if (!scheduleContainer) {
                throw new Error('Calendar schedule container (#schedule) not found.');
            }

            // 2. Load html2canvas
            const html2canvasFn = await loadHtml2Canvas();

            // Ensure web fonts are rendered
            if (document.fonts && document.fonts.ready) {
                await document.fonts.ready;
            }

            const metadata = getScheduleMetadata();

            // Detect active system theme dynamically
            const isDark = document.documentElement.classList.contains('dark') ||
                           localStorage.getItem('stms-theme') === 'dark';

            // 3. Render DOM to image canvas using html2canvas (Dynamic system theme background)
            const canvas = await html2canvasFn(scheduleContainer, {
                scale: 2, // 2x Retina scale suitable for 1080px social posts
                backgroundColor: isDark ? '#111827' : '#FFFFFF',
                useCORS: true,
                logging: false,
                windowWidth: 1200, // Force desktop width breakpoint in clone
                onclone: (clonedDoc, clonedElement) => {
                    prepareCloneForSocialExport(clonedDoc, clonedElement, metadata, isDark);
                }
            });

            // 4. Download canvas as PNG named SeriesTKD-Schedule-YYYY-MM-DD.png
            const filename = `SeriesTKD - Schedule - ${ metadata.anchorDate }.png`;
            const imageURL = canvas.toDataURL('image/png');
            const downloadLink = document.createElement('a');
            downloadLink.download = filename;
            downloadLink.href = imageURL;
            document.body.appendChild(downloadLink);
            downloadLink.click();
            document.body.removeChild(downloadLink);

        } catch (err) {
            console.error('[STMS Schedule Export Error]:', err);
            alert('Failed to generate schedule image. Please try again.');
        } finally {
            if (btn) {
                btn.innerHTML = originalContent;
                btn.disabled = originalDisabled;
                btn.removeAttribute('aria-busy');
            }
        }
    }

    return {
        exportScheduleSnapshot,
        loadHtml2Canvas,
        getScheduleMetadata,
        prepareCloneForSocialExport,
        to12Hour,
        formatTimeRange12H
    };
}));
