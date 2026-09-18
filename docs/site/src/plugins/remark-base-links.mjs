// Prefixes root-relative Markdown links ("/entities/overview/") with the site
// base ("/clicky"), so content can link by site path and still resolve when the
// site is served from a sub-path such as GitHub Pages project sites. Astro does
// not rewrite links inside Markdown content for `base`.
export default function remarkBaseLinks({ base }) {
  const prefix = base.replace(/\/+$/, '');
  if (prefix === '') {
    return () => {};
  }
  const rewrite = (node) => {
    if ((node.type === 'link' || node.type === 'definition') && isRootRelative(node.url, prefix)) {
      node.url = prefix + node.url;
    }
    for (const child of node.children ?? []) {
      rewrite(child);
    }
  };
  return (tree) => rewrite(tree);
}

function isRootRelative(url, prefix) {
  return (
    typeof url === 'string' &&
    url.startsWith('/') &&
    !url.startsWith('//') &&
    url !== prefix &&
    !url.startsWith(prefix + '/')
  );
}
