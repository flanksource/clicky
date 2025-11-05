// Global Tippy.js initialization function
function initTooltips(container) {
    const target = container || document.body;
    // Use singleton for better performance with many tooltips
    tippy('[title]', {
        content(reference) {
            const title = reference.getAttribute('title');
            reference.removeAttribute('title');
            reference.classList.add('tooltip-target');
            return title;
        },
        allowHTML: true,
        theme: 'light-border',
        placement: 'top',
        arrow: true,
        animation: 'shift-away',
        duration: [200, 150],
        delay: [0, 0],
        appendTo: () => document.body,
    });
}

// Initialize tooltips on page load
document.addEventListener('DOMContentLoaded', function() {
    initTooltips();
});
