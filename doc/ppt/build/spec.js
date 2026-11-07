// Shared slide spec: rendered by build-html.js and build-pptx.js. Units: inches, points. Canvas 10 x 5.625.
const P = {
  bg: '0D1117', panel: '161C24', panel2: '202B37', line: '2B3644',
  text: 'F2F5F7', muted: '9DA9B6', mint: '2EE6A6', lime: 'A3E635',
  amber: 'FFB547', orange: 'FF8A4C', red: 'FF5D5D', dark: '0D1117',
};
const MONO = 'Courier New';
const TOTAL = 14;

const rect = (x, y, w, h, o = {}) => ({ k: 'rect', x, y, w, h, fill: o.fill || P.panel, line: o.line, lw: o.lw || 1, r: o.r == null ? 0.1 : o.r });
const text = (x, y, w, h, paras, o = {}) => ({
  k: 'text', x, y, w, h, paras, size: o.size || 14, color: o.color || P.text, bold: !!o.bold, font: o.font,
  align: o.align || 'left', valign: o.valign || 'top', gap: o.gap || 0, italic: !!o.italic,
});
const icon = (name, x, y, s, color = P.mint) => ({ k: 'icon', name, x, y, s, color });
const circ = (name, x, y, d, o = {}) => [
  rect(x, y, d, d, { fill: o.fill || P.panel2, r: d / 2 }),
  icon(name, x + d * 0.24, y + d * 0.24, d * 0.52, o.color || P.mint),
];
const arrow = (x1, y1, x2, y2, color = P.muted, dash = false) => ({ k: 'arrow', x1, y1, x2, y2, color, dash });
const chip = (x, y, w, h, label, fill, tcolor = P.dark, size = 11) => [
  rect(x, y, w, h, { fill, r: h / 2 }),
  text(x, y, w, h, label, { size, bold: true, color: tcolor, align: 'center', valign: 'middle' }),
];
const label = (s) => text(0.5, 0.3, 6, 0.25, s, { size: 10, color: P.mint, font: MONO, bold: true });
const title = (s) => text(0.5, 0.55, 9, 0.7, s, { size: 36, bold: true });
const sub = (s) => text(0.5, 1.2, 9, 0.35, s, { size: 16, color: P.muted });
const footer = (n) => [
  text(0.5, 5.22, 3, 0.25, 'SafeShell', { size: 10, color: P.muted }),
  text(8.5, 5.22, 1, 0.25, `${n} / ${TOTAL}`, { size: 10, color: P.muted, align: 'right' }),
];
// terminal paragraph helper: runs [[text,color,bold]]
const T = (...runs) => runs.map((r) => (typeof r === 'string' ? { t: r } : { t: r[0], c: r[1], b: r[2] }));
const dots = (x, y) => [
  rect(x, y, 0.11, 0.11, { fill: P.red, r: 0.055 }),
  rect(x + 0.2, y, 0.11, 0.11, { fill: P.amber, r: 0.055 }),
  rect(x + 0.4, y, 0.11, 0.11, { fill: P.mint, r: 0.055 }),
];

const slides = [];

// 1 Title
slides.push({
  notes: 'Introduce ourselves and the project. SafeShell is our working name for a safety layer that lives inside the terminal. The terminal on the right shows the core idea: a dangerous command is stopped, explained, and a safer option is offered.',
  els: [
    text(0.5, 0.6, 5, 0.3, 'MINOR PROJECT  /  SEMESTER 7', { size: 11, color: P.mint, font: MONO, bold: true }),
    text(0.5, 1.0, 5.2, 1.0, 'SafeShell', { size: 54, bold: true }),
    text(0.5, 2.0, 5.0, 1.2, 'Transactional Command Framework with AI-Generated Undo Plans and Simulation-Based Safety Guarantees', { size: 18, color: P.muted }),
    text(0.5, 3.3, 5.2, 0.4, 'A safety layer that lives in your terminal.', { size: 16, color: P.text, bold: true }),
    rect(0.5, 4.0, 2.75, 0.95), ...circ('FaUser', 0.65, 4.2, 0.5),
    text(1.3, 4.1, 1.9, 0.35, 'Samarth Vala', { size: 16, bold: true, valign: 'middle' }),
    text(1.3, 4.45, 1.9, 0.3, '23BCP422 | Div 7 | G13', { size: 12, color: P.muted }),
    rect(3.45, 4.0, 2.75, 0.95), ...circ('FaUser', 3.6, 4.2, 0.5),
    text(4.25, 4.1, 1.9, 0.35, 'Tavish Gupta', { size: 16, bold: true, valign: 'middle' }),
    text(4.25, 4.45, 1.9, 0.3, '23BCP064 | Div 1 | G2', { size: 12, color: P.muted }),
    rect(6.45, 0.6, 3.05, 3.5, { fill: P.panel, line: P.line, r: 0.14 }), ...dots(6.65, 0.8),
    text(6.65, 1.2, 2.7, 3.6, [
      T(['$ rm -rf ~/projects', P.text]),
      T(''),
      T(['SAFESHELL: BLOCKED', P.red, true]),
      T(['Deletes 4,312 files', P.text]),
      T(['(2.1 GB, permanent)', P.muted]),
      T(''),
      T(['1) trash-put ~/projects', P.mint]),
      T(['   safer, recoverable', P.muted]),
      T(['2) Do it anyway + snapshot', P.text]),
      T(['3) Cancel', P.text]),
    ], { size: 12, font: MONO }),
  ],
});

// 2 AI agents
const chipRows = [['Claude Code', 1.85], ['Codex', 2.4], ['Other AI agents', 2.95]];
slides.push({
  notes: 'Today a large share of terminal commands are not typed by humans. Tools like Claude Code and Codex run commands for us. Nobody reads every one. The terminal itself has no checkpoint: it trusts whatever it is given.',
  els: [
    label('01  THE PROBLEM'), title('Who is typing your commands?'),
    sub('AI agents now run terminal commands for us, faster than we can read them.'),
    ...chipRows.flatMap(([n, y]) => [rect(0.5, y, 2.3, 0.45, { fill: P.panel2, r: 0.1 }), icon('FaRobot', 0.65, y + 0.11, 0.24, P.mint), text(1.0, y, 1.75, 0.45, n, { size: 14, bold: true, valign: 'middle' })]),
    ...chipRows.map(([, y]) => arrow(2.8, y + 0.22, 3.8, 2.6, P.muted)),
    rect(3.8, 1.85, 2.0, 1.55, { fill: P.panel, line: P.red, r: 0.12 }),
    icon('FaTerminal', 4.6, 2.0, 0.4, P.text),
    text(3.8, 2.5, 2.0, 0.35, 'Your terminal', { size: 16, bold: true, align: 'center' }),
    ...chip(4.1, 2.95, 1.4, 0.3, 'NO SAFETY CHECK', P.red, P.dark, 10),
    arrow(5.8, 2.62, 6.7, 2.62, P.muted),
    rect(6.7, 1.85, 2.8, 1.55, { fill: P.panel, line: P.line, r: 0.12 }),
    icon('FaServer', 7.9, 2.0, 0.4, P.amber),
    text(6.85, 2.5, 2.5, 0.8, 'Your files, company servers, production', { size: 14, align: 'center' }),
    ...[['FaBolt', 'Too fast', 'One task can run dozens of commands in seconds.'], ['FaEyeSlash', 'Too many', 'Nobody reads every command. Approvals become habit.'], ['FaBan', 'Too final', 'rm, dd, force-push: no undo, no recycle bin.']].flatMap(([ic, h, b], i) => {
      const x = 0.5 + i * 3.05;
      return [rect(x, 3.6, 2.9, 1.5), ...circ(ic, x + 0.2, 3.72, 0.5), text(x + 0.85, 3.72, 1.9, 0.5, h, { size: 16, bold: true, valign: 'middle' }), text(x + 0.2, 4.32, 2.55, 0.75, b, { size: 14, color: P.muted })];
    }),
    ...footer(2),
  ],
});

// 3 DevOps
slides.push({
  notes: 'In DevOps and on company servers the stakes are higher. This is an illustrative scenario, not a real incident: an agent is asked to clean logs, runs a command with the wrong path, and production data is gone. Humans cannot watch every server every minute, and approval prompts turn into automatic clicks.',
  els: [
    label('01  THE PROBLEM'), title('DevOps and company servers'),
    sub('Here, one wrong command means an outage.'),
    text(0.5, 1.62, 3, 0.25, 'Illustrative scenario', { size: 10, color: P.muted, italic: true }),
    ...[['FaRobot', P.mint, 'An AI agent is asked to clean old logs', null], ['FaTerminal', P.red, 'rm -rf /var/data', 'wrong path, no warning'], ['FaServer', P.amber, 'Production database is gone', 'customers affected']].flatMap(([ic, col, h, s], i) => {
      const x = 0.5 + i * 3.2;
      return [
        rect(x, 1.95, 2.6, 1.7, { fill: P.panel, line: i === 1 ? P.red : P.line }),
        ...circ(ic, x + 0.2, 2.1, 0.55, { color: col }),
        text(x + 0.2, 2.8, 2.2, 0.55, h, { size: 14, bold: true, font: i === 1 ? MONO : undefined, color: i === 1 ? P.red : P.text }),
        ...(s ? [text(x + 0.2, 3.38, 2.2, 0.25, s, { size: 12, color: P.muted })] : []),
      ];
    }),
    arrow(3.15, 2.8, 3.65, 2.8, P.muted), arrow(6.35, 2.8, 6.85, 2.8, P.muted),
    rect(0.5, 3.95, 4.4, 1.0), ...circ('FaEyeSlash', 0.65, 4.2, 0.5),
    text(1.3, 4.0, 3.45, 0.9, 'Humans cannot watch every server, every minute.', { size: 14, valign: 'middle' }),
    rect(5.1, 3.95, 4.4, 1.0), ...circ('FaCheckDouble', 5.25, 4.2, 0.5, { color: P.amber }),
    text(5.9, 4.0, 3.45, 0.9, 'Approval fatigue: people click Yes without reading.', { size: 14, valign: 'middle' }),
    ...footer(3),
  ],
});

// 4 Linux is hard
slides.push({
  notes: 'The second problem is human. Linux power comes with hundreds of commands and flags. To copy this week\'s PDFs you must remember a long find command. What the person actually means is one plain sentence. That gap keeps beginners away and makes experts slow.',
  els: [
    label('01  THE PROBLEM'), title('Linux asks you to remember'),
    rect(0.5, 1.45, 4.4, 1.95, { fill: P.panel, line: P.line }),
    ...chip(0.7, 1.62, 0.9, 0.28, 'TODAY', P.amber, P.dark, 10),
    text(0.7, 2.1, 4.0, 1.1, [T(['find . -name "*.pdf" \\', P.amber]), T(['  -mtime -7 -exec cp {} \\', P.amber]), T(['  ~/Documents/ \\;', P.amber])], { size: 13, font: MONO }),
    rect(5.1, 1.45, 4.4, 1.95, { fill: P.panel, line: P.mint }),
    ...chip(5.3, 1.62, 1.5, 0.28, 'WHAT YOU MEAN', P.mint, P.dark, 10),
    text(5.3, 2.1, 4.0, 1.1, '"Copy this week\'s PDFs to Documents"', { size: 22, bold: true, color: P.mint }),
    ...[['FaBook', 'Hundreds of commands and flags to remember'], ['FaKeyboard', 'One typo can delete the wrong thing'], ['FaGraduationCap', 'Beginners are afraid of the terminal']].flatMap(([ic, t], i) => {
      const x = 0.5 + i * 3.05;
      return [...circ(ic, x, 3.75, 0.55), text(x + 0.7, 3.65, 2.2, 0.9, t, { size: 14, valign: 'middle' })];
    }),
    text(0.5, 4.7, 9, 0.35, 'Today\'s people expect to just say what they want.', { size: 16, color: P.muted, italic: true }),
    ...footer(4),
  ],
});

// 5 Idea
slides.push({
  notes: 'Our answer is SafeShell: installed once, active in every terminal. It does four things: protect by blocking harmful commands with an explanation, prove by simulating in a sandbox, translate plain English into commands, and undo through snapshots. Important: the user always has the final say.',
  els: [
    label('02  OUR IDEA'), title('Our idea: SafeShell'),
    sub('Installed once, active in every terminal you open.'),
    ...[['FaShieldAlt', 'Protect', 'Blocks harmful commands and explains why.'], ['FaFlask', 'Prove', 'Runs the command in a sandbox first and shows the real impact.'], ['FaLanguage', 'Translate', 'Understands plain English. No need to remember commands.'], ['FaUndo', 'Undo', 'Snapshots let you roll back if something still goes wrong.']].flatMap(([ic, h, b], i) => {
      const x = 0.5 + i * 2.3;
      return [rect(x, 1.85, 2.1, 2.75), ...circ(ic, x + 0.2, 2.05, 0.7), text(x + 0.2, 2.95, 1.7, 0.4, h, { size: 20, bold: true }), text(x + 0.2, 3.4, 1.7, 1.15, b, { size: 14, color: P.muted })];
    }),
    text(0.5, 4.75, 9, 0.35, 'You always have the final say: SafeShell warns, it never locks you out.', { size: 16, bold: true, color: P.mint }),
    ...footer(5),
  ],
});

// 6 How it works
const steps = [['You type', 'A command or plain English'], ['Understand', 'Parse it and rate its risk'], ['Simulate', 'Run it in a safe sandbox'], ['Explain', 'Show impact and safer options'], ['Decide', 'Run, snapshot, or cancel']];
slides.push({
  notes: 'This is the flow for every command. The shell hook captures what you type, a background daemon understands and rates it, simulates it in a sandbox, explains the result, and you decide. Safe commands pass in milliseconds so the terminal never feels slow. Built with Rust and standard Linux features.',
  els: [
    label('03  HOW IT WORKS'), title('From keystroke to decision'),
    ...steps.flatMap(([h, b], i) => {
      const x = 0.5 + i * 1.84;
      return [
        rect(x, 1.5, 1.64, 2.1),
        rect(x + 0.15, 1.65, 0.42, 0.42, { fill: P.mint, r: 0.21 }),
        text(x + 0.15, 1.65, 0.42, 0.42, String(i + 1), { size: 16, bold: true, color: P.dark, align: 'center', valign: 'middle' }),
        text(x + 0.15, 2.2, 1.4, 0.35, h, { size: 16, bold: true }),
        text(x + 0.15, 2.6, 1.38, 0.95, b, { size: 14, color: P.muted }),
        ...(i < 4 ? [arrow(x + 1.64, 2.55, x + 1.84, 2.55, P.mint)] : []),
      ];
    }),
    rect(0.5, 3.8, 9, 0.55, { fill: P.panel2 }), ...[icon('FaBolt', 0.7, 3.96, 0.24, P.amber)],
    text(1.1, 3.8, 8.3, 0.55, 'Always on: hooks into bash and zsh. Safe commands pass in milliseconds.', { size: 14, valign: 'middle' }),
    text(0.5, 4.62, 1.0, 0.3, 'BUILT WITH', { size: 10, color: P.muted, bold: true, valign: 'middle' }),
    ...[['Rust', 0.8], ['Linux namespaces', 1.7], ['OverlayFS', 1.15], ['seccomp', 1.0], ['llama.cpp', 1.2]].reduce((acc, [n, w]) => {
      acc.els.push(...chip(acc.x, 4.6, w, 0.34, n, P.panel2, P.text, 12)); acc.x += w + 0.1; return acc;
    }, { x: 1.55, els: [] }).els,
    ...footer(6),
  ],
});

// 7 Risk levels
const risks = [['R0', P.mint, 'Read-only', 'ls, cat, grep', 'Runs instantly'], ['R1', P.lime, 'Reversible', 'mv, cp, mkdir', 'Runs, undo recorded'], ['R2', P.amber, 'Recoverable', 'sed -i, small rm', 'Simulate + snapshot'], ['R3', P.orange, 'Irreversible', 'rm -rf, dd, git push -f', 'Blocked, explained'], ['R4', P.red, 'Catastrophic', 'rm -rf /, chmod -R 777 /', 'Hard block']];
slides.push({
  notes: 'Every command is placed in one of five classes by how reversible it is. Rules decide first; the AI may only raise a level, never lower it. Low levels run instantly, high levels are stopped and explained.',
  els: [
    label('03  HOW IT WORKS'), title('Not every command is equal'),
    ...risks.flatMap(([r, col, m, ex, act], i) => {
      const y = 1.4 + i * 0.72;
      return [
        rect(0.5, y, 9, 0.62, { fill: P.panel, r: 0.08 }),
        ...chip(0.62, y + 0.1, 0.7, 0.42, r, col, P.dark, 16),
        text(1.5, y, 2.3, 0.62, m, { size: 16, bold: true, valign: 'middle' }),
        text(3.8, y, 3.2, 0.62, ex, { size: 12, font: MONO, color: P.muted, valign: 'middle' }),
        text(7.1, y, 2.3, 0.62, act, { size: 14, color: col, bold: true, valign: 'middle' }),
      ];
    }),
    ...footer(7),
  ],
});

// 8 Sandbox
const layers = [['Command sees', 'real files + sandbox changes', P.panel, P.line], ['Sandbox layer', 'every change lands here', P.panel2, P.mint], ['Real files', 'never touched', P.panel, P.line]];
slides.push({
  notes: 'This is the key idea. Instead of guessing the impact, we run the command for real inside a copy-on-write sandbox using OverlayFS. Everything it changes lands in a separate layer, so the report is an observed difference, not a prediction. Real files stay untouched until the user approves.',
  els: [
    label('03  HOW IT WORKS'), title('Simulate first, then decide'),
    ...layers.flatMap(([h, b, f, l], i) => {
      const y = 1.5 + i * 1.05;
      return [rect(0.5, y, 4.4, 0.8, { fill: f, line: l }), text(0.7, y + 0.08, 4.0, 0.35, h, { size: 16, bold: true, valign: 'middle' }), text(0.7, y + 0.42, 4.0, 0.3, b, { size: 14, color: P.muted }), ...(i < 2 ? [arrow(2.7, y + 0.8, 2.7, y + 1.05, P.mint)] : [])];
    }),
    rect(5.5, 1.5, 4.0, 3.1, { fill: P.panel, line: P.mint, r: 0.14 }),
    text(5.75, 1.65, 3.5, 0.25, 'IMPACT REPORT', { size: 11, bold: true, color: P.mint, font: MONO }),
    text(5.75, 1.98, 3.5, 0.75, '4,312', { size: 44, bold: true, color: P.red }),
    text(5.75, 2.75, 3.5, 0.35, 'files would be deleted', { size: 14, color: P.muted }),
    text(5.75, 3.2, 3.5, 0.75, [T(['deleted  4,312', P.red]), T(['created        0', P.text]), T(['modified       0', P.text])], { size: 14, font: MONO }),
    text(5.75, 4.05, 3.5, 0.4, 'Observed, not predicted', { size: 18, bold: true, color: P.mint }),
    ...footer(8),
  ],
});

// 9 Live example
const L = [
  T(['$ rm -rf ~/projects /tmp/old', P.text]), T(''),
  T(['SAFESHELL  BLOCKED  (irreversible)', P.red, true]),
  T(['Why: permanently deletes 4,312 files', P.text]),
  T(['     (2.1 GB). Cannot be recovered.', P.muted]), T(''),
  T(['1) trash-put ~/projects /tmp/old', P.mint], ['  [safer]', P.muted]),
  T(['2) rm -rI ~/projects /tmp/old', P.mint], ['     [asks first]', P.muted]),
  T(['3) Do it anyway + snapshot', P.amber]),
  T(['4) Do it anyway, no snapshot', P.amber]),
  T(['5) Cancel', P.text]), T(''),
  T(['Choose [1-5]: _', P.text]),
];
slides.push({
  notes: 'A live-style example. The command is stopped, the reason is given in plain English based on what the sandbox observed, safer options come first, and Do it anyway is always there. The user stays in control; the snapshot option means even then they can roll back.',
  els: [
    label('04  EXAMPLE'), title('A harmful command, caught'),
    rect(0.5, 1.4, 5.9, 3.6, { fill: P.panel, line: P.line, r: 0.14 }), ...dots(0.7, 1.58),
    text(0.7, 1.95, 5.5, 2.95, L, { size: 12, font: MONO }),
    ...[['Why it stopped', P.red, 'Plain-English reason from what the sandbox saw.'], ['Better options', P.mint, 'A safer command is offered first.'], ['You decide', P.amber, '"Do it anyway" is always there, with a snapshot.']].flatMap(([h, c, b], i) => {
      const y = 1.4 + i * 1.23;
      return [rect(6.65, y, 2.85, 1.1), text(6.85, y + 0.1, 2.5, 0.3, h, { size: 14, bold: true, color: c }), text(6.85, y + 0.43, 2.5, 0.65, b, { size: 14, color: P.muted })];
    }),
    ...footer(9),
  ],
});

// 10 Beyond files
const eff = [['FaFolderOpen', 'Files', 'Real diff from the sandbox'], ['FaNetworkWired', 'Network', 'Connections logged only'], ['FaMicrochip', 'Processes', 'Lists what would be killed'], ['FaDatabase', 'Databases', 'Run, rolled back, or cloned'], ['FaCodeBranch', 'Git and packages', 'Uses each tool\'s own dry run'], ['FaHdd', 'Disks and devices', 'Shows which drive is erased']];
slides.push({
  notes: 'A common question: a filesystem sandbox only sees files. So each kind of side effect has its own safeguard. Network calls are logged but never sent, processes are listed before killing, databases run in a rolled-back transaction or a clone, tools with their own dry run use it. Every report says how we know: observed, previewed, inspected or only estimated. Anything only estimated never runs automatically.',
  els: [
    label('05  BEYOND FILES'), title('Every side effect, covered'),
    sub('Files are only one part. Each kind of effect has its own safeguard.'),
    ...eff.flatMap(([ic, h, b], i) => {
      const x = 0.5 + (i % 3) * 3.05, y = 1.7 + Math.floor(i / 3) * 1.27;
      return [rect(x, y, 2.9, 1.15), ...circ(ic, x + 0.15, y + 0.12, 0.5), text(x + 0.8, y + 0.12, 2.0, 0.5, h, { size: 16, bold: true, valign: 'middle' }), text(x + 0.15, y + 0.7, 2.65, 0.4, b, { size: 14, color: P.muted })];
    }),
    text(0.5, 4.38, 6, 0.25, 'EVIDENCE SHOWN FOR EACH EFFECT', { size: 10, color: P.muted, bold: true }),
    ...[['OBSERVED', P.mint], ['PREVIEWED', P.lime], ['INSPECTED', P.amber], ['ESTIMATED', P.red]].flatMap(([n, c], i) => chip(0.5 + i * 1.5, 4.68, 1.4, 0.34, n, c, P.dark, 11)),
    text(6.6, 4.62, 2.9, 0.45, 'Estimated-only commands never run automatically.', { size: 12, color: P.muted, valign: 'middle' }),
    ...footer(10),
  ],
});

// 11 Natural language
slides.push({
  notes: 'Natural language mode. The user types what they want, SafeShell turns it into the exact command and shows it, so people learn as they use it. Plain English gives no extra power: delete everything is still rated and blocked. Voice control later sits on top of this same pipeline.',
  els: [
    label('06  NATURAL LANGUAGE'), title('Just say what you want'),
    rect(0.5, 1.5, 5.3, 0.5, { fill: P.panel2, r: 0.25 }),
    text(0.75, 1.5, 4.9, 0.5, 'copy report.pdf from downloads to documents', { size: 14, valign: 'middle' }),
    rect(0.5, 2.1, 5.3, 0.6, { fill: P.panel, line: P.line }),
    text(0.7, 2.1, 3.6, 0.6, 'cp ~/Downloads/report.pdf ~/Documents/', { size: 11, font: MONO, color: P.mint, valign: 'middle' }),
    ...chip(4.55, 2.25, 1.1, 0.3, 'SAFE', P.mint, P.dark, 11),
    rect(0.5, 3.1, 5.3, 0.5, { fill: P.panel2, r: 0.25 }),
    text(0.75, 3.1, 4.9, 0.5, 'delete everything in this folder', { size: 14, valign: 'middle' }),
    rect(0.5, 3.7, 5.3, 0.6, { fill: P.panel, line: P.line }),
    text(0.7, 3.7, 3.6, 0.6, 'rm -rf ./*', { size: 12, font: MONO, color: P.red, valign: 'middle' }),
    ...chip(4.3, 3.85, 1.35, 0.3, 'R3 BLOCKED', P.red, P.dark, 11),
    ...[['FaEye', 'Shows the command', 'You learn as you use it.'], ['FaQuestionCircle', 'Asks when unclear', '3 files match: which one?'], ['FaLock', 'Same safety rules', 'Plain English gives no extra power.']].flatMap(([ic, h, b], i) => {
      const y = 1.5 + i * 1.15;
      return [...circ(ic, 6.1, y + 0.1, 0.55), text(6.85, y, 2.65, 0.4, h, { size: 16, bold: true, valign: 'middle' }), text(6.85, y + 0.4, 2.65, 0.6, b, { size: 14, color: P.muted })];
    }),
    ...circ('FaMicrophone', 0.5, 4.5, 0.5, { color: P.amber }),
    text(1.15, 4.5, 8, 0.5, 'Next: voice control on top of the same pipeline.', { size: 14, color: P.muted, valign: 'middle' }),
    ...footer(11),
  ],
});

// 12 AI layer
slides.push({
  notes: 'The AI is optional and swappable. Off: rules only. Local: runs on the user machine, private and offline, with a Qwen model the user picks from the /models menu; SafeShell downloads and runs the engine itself on a CUDA GPU, any Vulkan GPU, or the CPU. Cloud: the user pastes their own key for Claude, GPT or Gemini. Hybrid: local first, cloud only if allowed. Key safety principle: AI proposes, sandbox verifies. AI can raise risk but never lower it.',
  els: [
    label('07  THE AI LAYER'), title('AI: local or cloud, your choice'),
    ...[['FaPowerOff', 'Off', 'Rules only. Works fully offline.'], ['FaMicrochip', 'Local', 'Runs on your machine. Private and free.'], ['FaCloud', 'Cloud', 'Your own API key. Best on weak PCs.'], ['FaBalanceScale', 'Hybrid', 'Local first, cloud only if you allow.']].flatMap(([ic, h, b], i) => {
      const x = 0.5 + i * 2.3;
      return [rect(x, 1.4, 2.1, 2.2), ...circ(ic, x + 0.2, 1.55, 0.55), text(x + 0.2, 2.25, 1.7, 0.35, h, { size: 18, bold: true }), text(x + 0.2, 2.68, 1.7, 0.9, b, { size: 14, color: P.muted })];
    }),
    rect(0.5, 3.85, 4.4, 1.2, { fill: P.panel2 }),
    text(0.7, 3.93, 4.0, 0.3, 'Local models', { size: 16, bold: true }),
    text(0.7, 4.27, 4.0, 0.75, 'Qwen models run by SafeShell itself on CUDA, Vulkan or CPU. Or connect Claude, GPT or Gemini.', { size: 14, color: P.muted }),
    rect(5.1, 3.85, 4.4, 1.2, { fill: P.panel2, line: P.mint }),
    text(5.3, 3.93, 4.0, 0.3, 'AI proposes, sandbox verifies', { size: 16, bold: true, color: P.mint }),
    text(5.3, 4.27, 4.0, 0.75, 'Undo plans are tested in the sandbox first. AI can raise risk, never lower it.', { size: 14, color: P.muted }),
    ...footer(12),
  ],
});

// 13 Results
const built = ['Foundation', 'Sandbox', 'Shell hook', 'AI layer', 'Undo', 'English input'];
const measured = [['92%', 'of 220 commands rated exactly, by an independent test set'], ['97%', 'of 43 plain-English requests turned into the right command (local Qwen 3B; Gemini 95-100%)'], ['0.5 s', 'to translate a request once the local model is loaded'], ['339', 'automated tests, incl. a real terminal and the real kernel']];
slides.push({
  notes: 'All six build phases are working. We measured the project. A held-out set of 220 commands, labelled by someone who had not seen our rules, is rated exactly 92 percent of the time, and every miss but one is within one level. We first scored 77 percent on it, then fixed the real gaps it exposed. On 43 plain-English requests the local Qwen 3B model produced a correct command 88 percent of the time, and its failures were fixed. 339 automated tests pass, including tests in a real terminal and against the real Linux kernel.',
  els: [
    label('08  RESULTS'), title('What we built and measured'),
    ...built.flatMap((p, i) => {
      const x = 0.5 + i * 1.52;
      return [rect(x, 1.5, 1.4, 0.95), text(x + 0.12, 1.58, 1.2, 0.22, 'PHASE ' + (i + 1), { size: 10, color: P.mint, font: MONO, bold: true }), text(x + 0.12, 1.85, 1.2, 0.55, p, { size: 14, bold: true })];
    }),
    text(0.5, 2.75, 6, 0.35, 'Measured, not promised', { size: 18, bold: true }),
    ...measured.flatMap(([b, c], i) => {
      const x = 0.5 + i * 2.3;
      return [rect(x, 3.2, 2.1, 1.75, { fill: P.panel }), text(x + 0.2, 3.4, 1.75, 0.5, b, { size: 24, bold: true, color: P.mint }), text(x + 0.2, 4.05, 1.75, 0.85, c, { size: 13, color: P.muted })];
    }),
    ...footer(13),
  ],
});

// 14 Future + thanks
slides.push({
  notes: 'Future scope: voice control, native btrfs and ZFS snapshots, more tool previews, team policies for labs and servers, and a learning mode. Close by thanking the faculty and inviting questions.',
  els: [
    label('09  FUTURE'), title('Where this goes next'),
    ...[['FaMicrophone', 'Voice control', 'Speak it, same safety checks.'], ['FaCodeBranch', 'More tools', 'git, docker, kubectl.'], ['FaUsers', 'Team policies', 'Shared rules for labs and servers.'], ['FaGraduationCap', 'Learning mode', 'Teaches real commands.']].flatMap(([ic, h, b], i) => {
      const x = 0.5 + i * 2.3;
      return [rect(x, 1.45, 2.1, 1.9), ...circ(ic, x + 0.2, 1.6, 0.55), text(x + 0.2, 2.25, 1.7, 0.35, h, { size: 16, bold: true }), text(x + 0.2, 2.62, 1.7, 0.7, b, { size: 14, color: P.muted })];
    }),
    rect(0.5, 3.65, 9, 1.4, { fill: P.panel2, r: 0.14 }),
    text(0.75, 3.8, 4.2, 0.65, 'Thank you', { size: 36, bold: true }),
    text(0.75, 4.45, 4.2, 0.4, 'Questions?', { size: 20, bold: true, color: P.mint }),
    text(5.2, 3.9, 4.1, 0.5, [T(['Samarth Vala', P.text, true]), T(['23BCP422 | Div 7 | G13', P.muted])], { size: 14 }),
    text(5.2, 4.4, 4.1, 0.5, [T(['Tavish Gupta', P.text, true]), T(['23BCP064 | Div 1 | G2', P.muted])], { size: 14 }),
    ...footer(14),
  ],
});

module.exports = { P, MONO, slides };
