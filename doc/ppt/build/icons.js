const React = require('react');
const { renderToStaticMarkup } = require('react-dom/server');
const fa = require('react-icons/fa');
function svg(name, color, px = 256) {
  const C = fa[name];
  if (!C) throw new Error('missing icon ' + name);
  return renderToStaticMarkup(React.createElement(C, { color: '#' + color, size: px }));
}
module.exports = { svg };
