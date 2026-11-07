const fs = require('fs');
const { P, MONO, slides } = require('./spec');
const { svg } = require('./icons');
const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
const norm = (p) => (typeof p === 'string' ? [[{ t: p }]] : p.map((q) => (typeof q === 'string' ? [{ t: q }] : q)));
const f = (n) => +n.toFixed(3) + 'in';
function el(e) {
  if (e.k === 'rect') {
    const b = e.line ? `border:${e.lw}pt solid #${e.line};` : '';
    return `<div style="left:${f(e.x)};top:${f(e.y)};width:${f(e.w)};height:${f(e.h)};background:#${e.fill};border-radius:${f(Math.min(e.r, Math.min(e.w, e.h) / 2))};${b}"></div>`;
  }
  if (e.k === 'icon') return `<div class="ic" style="left:${f(e.x)};top:${f(e.y)};width:${f(e.s)};height:${f(e.s)}">${svg(e.icon || e.name, e.color, 64).replace('<svg', '<svg style="width:100%;height:100%"')}</div>`;
  if (e.k === 'text') {
    const jc = { top: 'flex-start', middle: 'center', bottom: 'flex-end' }[e.valign];
    const font = e.font ? `font-family:'${e.font}','Liberation Mono',monospace;` : '';
    const ps = norm(e.paras).map((runs) => `<p style="margin:0 0 ${e.gap}pt 0;min-height:1.22em;white-space:pre-wrap">${runs.map((r) => `<span style="${r.c ? 'color:#' + r.c + ';' : ''}${r.b ? 'font-weight:700;' : ''}">${esc(r.t)}</span>`).join('')}</p>`).join('');
    return `<div class="tx" style="left:${f(e.x)};top:${f(e.y)};width:${f(e.w)};height:${f(e.h)};justify-content:${jc};text-align:${e.align};font-size:${e.size}pt;color:#${e.color};${e.bold ? 'font-weight:700;' : ''}${e.italic ? 'font-style:italic;' : ''}${font}">${ps}</div>`;
  }
  return '';
}
function arrow(e) {
  const dx = e.x2 - e.x1, dy = e.y2 - e.y1, L = Math.hypot(dx, dy), ux = dx / L, uy = dy / L;
  const hl = 0.13, hw = 0.065, bx = e.x2 - ux * hl, by = e.y2 - uy * hl;
  const pts = `${e.x2},${e.y2} ${bx - uy * hw},${by + ux * hw} ${bx + uy * hw},${by - ux * hw}`;
  return `<line x1="${e.x1}" y1="${e.y1}" x2="${bx}" y2="${by}" stroke="#${e.color}" stroke-width="0.022"${e.dash ? ' stroke-dasharray="0.06"' : ''}/><polygon points="${pts}" fill="#${e.color}"/>`;
}
const sections = slides.map((s, i) => {
  const arrows = s.els.filter((e) => e.k === 'arrow').map(arrow).join('');
  return `<section class="slide" id="s${i + 1}"><div class="bg"></div>${s.els.filter((e) => e.k !== 'arrow').map(el).join('')}<svg class="ar" viewBox="0 0 10 5.625">${arrows}</svg></section>`;
}).join('\n');
const html = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>SafeShell Presentation</title>
<style>
:root{--bg:#${P.bg}}
*{box-sizing:border-box}
html,body{margin:0;height:100%;background:#05080c;overflow:hidden;font-family:Calibri,Carlito,'Segoe UI',Arial,sans-serif}
#stage{position:absolute;left:50%;top:50%;width:10in;height:5.625in;transform-origin:0 0}
.slide{position:absolute;inset:0;width:10in;height:5.625in;background:var(--bg);overflow:hidden;display:none;line-height:1.22}
.slide.on{display:block}
.slide>div,.slide>svg{position:absolute}
.bg{inset:0;background:var(--bg)}
.tx{display:flex;flex-direction:column;color:#fff;overflow:visible;white-space:normal}
.ic{display:block}
.ar{left:0;top:0;width:10in;height:5.625in}
#hint{position:fixed;right:12px;bottom:8px;color:#5d6b7a;font:12px Calibri,Carlito,sans-serif;z-index:5}
@media print{@page{size:10in 5.625in;margin:0}html,body{overflow:visible;height:auto;background:none}#stage{position:static;transform:none!important}.slide{display:block;position:relative;page-break-after:always}#hint{display:none}}
</style></head><body>
<div id="stage">
${sections}
</div>
<div id="hint">Arrow keys or click to move · F fullscreen · Print to save as PDF</div>
<script>
const slides=[...document.querySelectorAll('.slide')],stage=document.getElementById('stage');
let i=Math.max(0,Math.min(slides.length-1,(parseInt(location.hash.slice(1))||1)-1));
function fit(){const s=Math.min(innerWidth/960,innerHeight/540);stage.style.transform='scale('+s+') translate(-50%,-50%)';}
function show(n){i=Math.max(0,Math.min(slides.length-1,n));slides.forEach((s,k)=>s.classList.toggle('on',k===i));history.replaceState(null,'','#'+(i+1));}
addEventListener('resize',fit);
addEventListener('keydown',e=>{if(['ArrowRight','ArrowDown',' ','PageDown'].includes(e.key)){e.preventDefault();show(i+1)}else if(['ArrowLeft','ArrowUp','PageUp'].includes(e.key)){e.preventDefault();show(i-1)}else if(e.key==='Home')show(0);else if(e.key==='End')show(slides.length-1);else if(e.key==='f'||e.key==='F'){document.fullscreenElement?document.exitFullscreen():document.documentElement.requestFullscreen()}});
addEventListener('click',e=>{show(e.clientX>innerWidth/2?i+1:i-1)});
fit();show(i);
</script></body></html>`;
fs.writeFileSync(process.argv[2] || 'safeshell.html', html);
console.log('html ok', html.length);
