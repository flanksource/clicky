// Tree expand/collapse Alpine.js data function
function createTreeData() {
    return {
        expandedNodes: new Set(),
        expandAll() {
            const nodes = this.$el.querySelectorAll('[data-node-id]');
            nodes.forEach(n => this.expandedNodes.add(n.dataset.nodeId));
        },
        collapseAll() {
            this.expandedNodes.clear();
        },
        toggleNode(id) {
            if (this.expandedNodes.has(id)) {
                this.expandedNodes.delete(id);
            } else {
                this.expandedNodes.add(id);
            }
        },
        isExpanded(id) {
            return this.expandedNodes.has(id);
        }
    };
}
