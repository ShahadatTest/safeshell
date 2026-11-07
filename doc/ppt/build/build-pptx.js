const pptxgen = require('pptxgenjs');
const sharp = require('sharp');
const { P, MONO, slides } = require('./spec');
const { svg } = require('./icons');
const norm = (p) => (typeof p === 'string' ? [[{ t: p }]] : p.map((q) => (typeof q === 'string' ? [{ t: q }] : q)));
(async () => {
  const pres = new pptxgen();
  pres.layout = 'LAYOUT_16x9';
  pres.title = 'SafeShell - Transactional Command Framework';
  pres.author = 'Samarth Vala, Tavish Gupta';
  const cache = {};
  const png = async (name, color) => {
    const k = name + color;
    if (!cache[k]) cache[k] = 'image/png;base64,' + (await sharp(Buffer.from(svg(name, color, 256))).resize(256, 256).png().toBuffer()).toString('base64');
    return cache[k];
  };
  for (const s of slides) {
    const sl = pres.addSlide();
    sl.background = { color: P.bg };
    if (s.notes) sl.addNotes(s.notes);
    for (const e of s.els) {
      if (e.k === 'rect') {
        const r = Math.min(e.r, Math.min(e.w, e.h) / 2);
        const o = { x: e.x, y: e.y, w: e.w, h: e.h, fill: { color: e.fill } };
        if (e.line) o.line = { color: e.line, width: e.lw }; else o.line = { color: e.fill, width: 0 };
        if (r > 0.001) { o.rectRadius = r; sl.addShape(pres.ShapeType.roundRect, o); } else sl.addShape(pres.ShapeType.rect, o);
      } else if (e.k === 'icon') {
        sl.addImage({ data: await png(e.name, e.color), x: e.x, y: e.y, w: e.s, h: e.s });
      } else if (e.k === 'arrow') {
        const x = Math.min(e.x1, e.x2), y = Math.min(e.y1, e.y2), w = Math.abs(e.x2 - e.x1), h = Math.abs(e.y2 - e.y1);
        const flipV = (e.y2 < e.y1) !== (e.x2 < e.x1);
        sl.addShape(pres.ShapeType.line, { x, y, w, h, flipH: e.x2 < e.x1 && e.y1 === e.y2, flipV: flipV && h > 0, line: { color: e.color, width: 1.6, endArrowType: 'triangle', dashType: e.dash ? 'dash' : 'solid' } });
      } else if (e.k === 'text') {
        const paras = norm(e.paras), runs = [];
        paras.forEach((ps, pi) => ps.forEach((r, ri) => {
          const last = ri === ps.length - 1 && pi < paras.length - 1;
          runs.push({ text: r.t === '' ? ' ' : r.t, options: { color: r.c || e.color, bold: r.b || e.bold, italic: e.italic, fontFace: e.font || 'Calibri', fontSize: e.size, breakLine: last } });
        }));
        sl.addText(runs, { x: e.x, y: e.y, w: e.w, h: e.h, margin: 0, align: e.align, valign: e.valign, isTextBox: true, fit: 'none', paraSpaceAfter: e.gap, wrap: true });
      }
    }
  }
  await pres.writeFile({ fileName: process.argv[2] || 'safeshell.pptx' });
  console.log('pptx ok');
})();
