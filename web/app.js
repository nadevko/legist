
/**
 * LEXDIFF app.js v2.0
 * БЭКЕНДЕРУ: все точки интеграции помечены // BACKEND:
 * API_BASE — заменить на реальный адрес сервера
 */

const CFG = {
  API_BASE: '/api',
  POLL_MS: 1500,
  TOAST_MS: 3000,
  DEBOUNCE_MS: 300,
};

/* ── MOCK DATA (удалить после подключения API) ── */
const ACTS = [
  {id:1,type:'ЛНА',title:'Правила внутреннего трудового распорядка',org:'ООО «ТехПром»',versions:5,date:'12.02.2025',risk:'red',   bar:['r','o','g','e','e']},
  {id:2,type:'ЛНА',title:'Положение об оплате труда',               org:'ООО «ТехПром»',versions:3,date:'05.11.2024',risk:'orange',bar:['e','o','g','e','e']},
  {id:3,type:'НПА',title:'Инструкция по охране труда № 14',          org:'ГУП «СтройПроект»',versions:7,date:'18.09.2024',risk:'green', bar:['e','e','g','g','e']},
  {id:4,type:'ЛНА',title:'Положение о премировании сотрудников',     org:'АО «БелТех»',versions:2,date:'22.01.2025',risk:'red',   bar:['r','o','e','e','e']},
  {id:5,type:'НПА',title:'Коллективный договор на 2025 год',         org:'ЗАО «МашЗавод»',versions:4,date:'01.03.2025',risk:'orange',bar:['e','o','g','e','e']},
  {id:6,type:'ЛНА',title:'Положение о командировочных расходах',     org:'ООО «ТехПром»',versions:2,date:'14.02.2025',risk:'green', bar:['e','e','g','g','e']},
  {id:7,type:'НПА',title:'Инструкция по документообороту',           org:'ГУП «СтройПроект»',versions:6,date:'03.12.2024',risk:'red',   bar:['r','r','e','e','e']},
  {id:8,type:'ЛНА',title:'Положение о персональных данных',          org:'АО «БелТех»',versions:3,date:'17.10.2024',risk:'orange',bar:['e','o','g','e','e']},
  {id:9,type:'ЛНА',title:'Должностная инструкция юриста',            org:'ЗАО «МашЗавод»',versions:2,date:'08.08.2024',risk:'green', bar:['e','e','g','g','e']},
  {id:10,type:'НПА',title:'Кодекс корпоративной этики',             org:'ООО «ТехПром»',versions:1,date:'30.06.2024',risk:'green', bar:['e','e','g','e','e']},
  {id:11,type:'ЛНА',title:'Положение о структурных подразделениях', org:'ООО «ТехПром»',versions:3,date:'10.01.2025',risk:'orange',bar:['e','o','e','e','e']},
  {id:12,type:'НПА',title:'Правила безопасности при работе с ПК',   org:'АО «БелТех»',versions:4,date:'25.03.2025',risk:'red',   bar:['r','o','e','e','e']},
  {id:13,type:'ЛНА',title:'Инструкция по делопроизводству',         org:'ЗАО «МашЗавод»',versions:2,date:'11.02.2025',risk:'green', bar:['e','e','g','e','e']},
  {id:14,type:'НПА',title:'Положение о защите коммерческой тайны',  org:'ООО «ТехПром»',versions:5,date:'07.03.2025',risk:'orange',bar:['e','o','g','e','e']},
  {id:15,type:'ЛНА',title:'Положение об аттестации работников',     org:'ГУП «СтройПроект»',versions:2,date:'19.02.2025',risk:'red',   bar:['r','e','g','e','e']},
];

const CHANGES = [
  {n:1, s:'П.2.4',    old:'Работник имеет право подать заявление в срок...',    nw:'Работник обязан подать заявление в течение...',          type:'Изменение обязанности',   risk:'red',    law:'ст.55 ТК РБ',           rec:'Проверить соответствие трудовому законодательству'},
  {n:2, s:'Статья 5', old:'Решение принимается руководителем организации...',   nw:'Решение принимается комиссией организации',              type:'Изменение субъекта',      risk:'orange', law:'Закон №300-З',           rec:'Проверить полномочия комиссии'},
  {n:3, s:'П.4.1',    old:'Заявление рассматривается до 10 дней',               nw:'Заявление рассматривается до 5 дней',                    type:'Изменение срока',         risk:'orange', law:'Закон №433-З',           rec:'Проверить достаточность срока'},
  {n:4, s:'П.6.3',    old:'Работодатель вправе отказать без объяснений...',     nw:'Работодатель обязан указать причины отказа',             type:'Изменение процедуры',     risk:'green',  law:'—',                     rec:'Изменение повышает прозрачность процедур'},
  {n:5, s:'Статья 12',old:'Работник обязан уведомить руководство',              nw:'Работник обязан уведомить руководство и отдел кадров',  type:'Добавление субъекта',     risk:'green',  law:'—',                     rec:'Проверить корректность распределения обязанностей'},
  {n:6, s:'П.7.2',    old:'Документы хранятся 3 года',                          nw:'Документы хранятся 5 лет',                              type:'Изменение срока',         risk:'orange', law:'Закон об архивном деле', rec:'Проверить соответствие нормативам хранения'},
  {n:7, s:'Раздел III',old:'Порядок регистрации заявлений',                     nw:'Раздел удалён',                                         type:'Удаление раздела',        risk:'red',    law:'Закон №130-З',           rec:'Проверить обязательность процедуры регистрации'},
  {n:8, s:'П.3.5',    old:'Работник может подать заявление в письменном виде',  nw:'Работник может подать заявление в письменном или эл. виде', type:'Добавление способа',  risk:'green',  law:'Закон об эл. документе', rec:'Изменение расширяет способы подачи'},
  {n:9, s:'П.8.1',    old:'Выплата производится ежемесячно',                    nw:'Выплата производится один раз в квартал',                type:'Изменение порядка выплат',risk:'red',   law:'ТК РБ ст.73',            rec:'Проверить допустимость изменения периода выплат'},
  {n:10,s:'П.9.2',    old:'Руководитель утверждает график отпусков',             nw:'Руководитель утверждает график после согласования с профсоюзом', type:'Изменение процедуры', risk:'orange', law:'ТК РБ ст.168', rec:'Проверить обязательность согласования'},
];

const HIERARCHY = [
  {n:'01',text:'Конституция Республики Беларусь',ok:true},
  {n:'02',text:'Решения республиканских референдумов',ok:true},
  {n:'03',text:'Законы Республики Беларусь',ok:false},
  {n:'04',text:'Декреты и указы Президента Республики Беларусь',ok:true},
  {n:'05',text:'Постановления Совета Министров Республики Беларусь',ok:true},
  {n:'06',text:'Постановления Палаты представителей и Совета Республики',ok:true},
  {n:'07',text:'НПА министерств и республиканских органов госуправления',ok:false},
  {n:'08',text:'Решения местных референдумов и Советов депутатов',ok:true},
  {n:'09',text:'НПА иных нормотворческих органов (должностных лиц)',ok:true},
  {n:'10',text:'Технические нормативные правовые акты',ok:true},
];

const VIOLATIONS = [
  {s:'П.2.4',    risk:'red',    text:'Замена «имеет право» → «обязан» противоречит ч.1 ст.55 ТК РБ — работник не может быть принуждён к действию без нормативного основания.', law:'ст.55 ТК РБ',         href:'https://pravo.by/document/?guid=3871&p0=hk9900296'},
  {s:'Раздел III',risk:'red',   text:'Удаление порядка регистрации заявлений нарушает ст.14 Закона №130-З — обязательная процедура регистрации должна быть закреплена в ЛНА.', law:'Закон №130-З',         href:'https://pravo.by'},
  {s:'П.8.1',    risk:'red',    text:'Изменение периодичности выплат с ежемесячной на квартальную нарушает ст.73 ТК РБ — заработная плата должна выплачиваться не реже 2 раз в месяц.', law:'ТК РБ ст.73', href:'https://pravo.by'},
  {s:'Статья 5', risk:'orange', text:'Передача полномочий от руководителя к комиссии требует проверки соответствия уставным документам и Закону №300-З.', law:'Закон №300-З',                               href:'https://pravo.by'},
  {s:'П.7.2',    risk:'orange', text:'Увеличение срока хранения документов требует проверки соответствия нормативам Закона об архивном деле РБ.', law:'Закон об архивном деле',                              href:'https://pravo.by'},
];

const CHAIN_VERSIONS = [
  {ver:'v5',date:'12.02.2025',author:'Иванов А.П.',risk:'red',   changes:10,red:3,org:4,grn:3,title:'Плановое обновление 2025'},
  {ver:'v4',date:'05.11.2024',author:'Петрова О.С.',risk:'orange',changes:6, red:1,org:3,grn:2,title:'Корректировка оплаты труда'},
  {ver:'v3',date:'14.07.2024',author:'Сидоров К.В.',risk:'orange',changes:4, red:0,org:2,grn:2,title:'Правки по результатам проверки'},
  {ver:'v2',date:'02.03.2024',author:'Иванов А.П.',risk:'green', changes:3, red:0,org:1,grn:2,title:'Техническая актуализация'},
  {ver:'v1',date:'10.01.2024',author:'Петрова О.С.',risk:'green', changes:0, red:0,org:0,grn:0,title:'Первоначальная редакция'},
];

/* ── STATE ── */
const S = {
  page: 'acts',
  view: 'grid',
  acts: {type:'all', risk:null, search:''},
  tbl:  {risk:'all', search:''},
  sort: {col:null, dir:'asc'},
  perPage: 10,
  ctxId: null,
  files: {old:null, new:null},
  jobId: null,
  poll: null,
  kbSeq: '',
  kbTimer: null,
};

/* ── INIT ── */
document.addEventListener('DOMContentLoaded', () => {
  renderGrid(ACTS);
  renderList(ACTS);
  renderTable(CHANGES);
  renderVTV();
  renderHierarchy();
  renderViolations();
  renderChainTimeline();
  renderChainChart();
  showPage('acts');

  /* Поиск с debounce */
  const si = document.getElementById('acts-search');
  if (si) {
    let dt;
    si.addEventListener('input', () => {
      clearTimeout(dt);
      dt = setTimeout(() => { S.acts.search = si.value.trim().toLowerCase(); applyActsFilter(); }, CFG.DEBOUNCE_MS);
    });
  }

  /* Закрытие контекстного меню */
  document.addEventListener('click', e => {
    if (!e.target.closest('.ctx-menu') && !e.target.closest('.dots-btn')) closeCtx();
  });

  /* VTV-бар закрытие */
  document.addEventListener('click', () => {
    const bar = document.getElementById('vtv-bar');
    if (bar) bar.classList.add('hidden');
  });

  /* Горячие клавиши */
  document.addEventListener('keydown', handleKbd);
});

/* ── ROUTER ── */
function showPage(name) {
  document.querySelectorAll('.page').forEach(p => p.classList.add('hidden'));
  const pg = document.getElementById('page-' + name);
  if (pg) pg.classList.remove('hidden');
  S.page = name;
  document.querySelectorAll('.sb-link[data-page], .topnav-btn[data-page]').forEach(el => {
    el.classList.toggle('active', el.dataset.page === name);
  });
}

/* ── SIDEBAR ── */
function toggleSbGroup(btn, subId) {
  const sub = document.getElementById(subId);
  if (!sub) return;
  btn.classList.toggle('open');
  sub.classList.toggle('collapsed');
}

/* ── MOBILE ── */
function toggleMobSidebar() {
  const sb = document.getElementById('sidebar');
  const ov = document.getElementById('mob-overlay');
  sb.classList.toggle('mob-open');
  ov.classList.toggle('hidden');
}

/* ── ACTS GRID ── */
function renderGrid(acts) {
  const el = document.getElementById('acts-grid');
  if (!el) return;
  el.innerHTML = acts.map(a => `
    <div class="act-card" onclick="openActDetailById(${a.id})">
      <div class="ac-top">
        <span class="ac-type">${a.type}</span>
        <span class="bdg ${rBdg(a.risk)}">${rLbl(a.risk)}</span>
      </div>
      <div class="ac-title">${a.title}</div>
      <div class="ac-org">${a.org}</div>
      <div class="ac-meta">
        <span class="ac-vers">${a.versions} ${pl(a.versions,'версия','версии','версий')}</span>
        <span class="ac-bar">+1+1 ${barHtml(a.bar)}</span>
      </div>
      <div class="ac-date">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" width="12" height="12"><rect x="3" y="4" width="18" height="18" rx="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>
        ${a.date}
      </div>
      <div class="ac-footer">
        <button class="btn-outline" style="font-size:12px;padding:5px 12px" onclick="event.stopPropagation();openActDetailById(${a.id})">Подробнее</button>
        <button class="dots-btn" onclick="event.stopPropagation();showCtx(event,${a.id})">···</button>
      </div>
    </div>`).join('');
}

function renderList(acts) {
  const el = document.getElementById('acts-list');
  if (!el) return;
  el.innerHTML = acts.map(a => `
    <div class="act-row" onclick="openActDetailById(${a.id})">
      <div class="ar-type"><span class="ac-type">${a.type}</span></div>
      <div class="ar-title">${a.title}</div>
      <div class="ar-org">${a.org}</div>
      <div class="ar-date">${a.date}</div>
      <div class="ar-bdg"><span class="bdg ${rBdg(a.risk)}">${rFull(a.risk)}</span></div>
      <button class="dots-btn" onclick="event.stopPropagation();showCtx(event,${a.id})">···</button>
    </div>`).join('');
}

function applyActsFilter() {
  /* BACKEND: заменить на GET /api/acts?type=&risk=&search= */
  let f = ACTS;
  const {type, risk, search} = S.acts;
  if (type && type !== 'all') f = f.filter(a => a.type === type);
  if (risk) f = f.filter(a => a.risk === risk);
  if (search) f = f.filter(a => a.title.toLowerCase().includes(search) || a.org.toLowerCase().includes(search));
  renderGrid(f);
  renderList(f);
  const el = document.getElementById('acts-pg-info');
  if (el) el.textContent = `Показано 1–${f.length} из ${f.length}`;
}

function filterActs(key, val, btn) {
  S.acts[key] = val;
  const gid = key === 'type' ? 'acts-type-filter' : 'acts-risk-filter';
  const g = document.getElementById(gid);
  if (g) { g.querySelectorAll('.fchip').forEach(c => c.classList.remove('active')); btn.classList.add('active'); }
  applyActsFilter();
}

function setView(v) {
  S.view = v;
  document.getElementById('acts-grid').classList.toggle('hidden', v !== 'grid');
  document.getElementById('acts-list').classList.toggle('hidden', v !== 'list');
  document.getElementById('vsw-grid').classList.toggle('active', v === 'grid');
  document.getElementById('vsw-list').classList.toggle('active', v === 'list');
}

function openActDetailById(id) {
  S.ctxId = id;
  const a = ACTS.find(x => x.id === id);
  if (!a) return;
  /* BACKEND: GET /api/acts/:id → Act, GET /api/acts/:id/versions → Version[] */
  document.getElementById('detail-title').textContent = a.title;
  document.getElementById('detail-sub').textContent = `${a.org} · ${a.versions} версий`;
  showPage('act-detail');
}

function selectAllVersions(cb) {
  document.querySelectorAll('.v-check').forEach(c => c.checked = cb.checked);
}

function actsPage(dir) { toast('Пагинация ' + (dir > 0 ? '→' : '←') + ' (требуется backend)'); }

/* ── UPLOAD ── */
function dzOver(e) { e.preventDefault(); e.currentTarget.classList.add('over'); }
function dzLeave(e) { e.currentTarget.classList.remove('over'); }
function dzDrop(e, side) { e.preventDefault(); e.currentTarget.classList.remove('over'); const f = e.dataTransfer.files[0]; if (f) handleFile(f, side); }
function fiSel(e, side) { const f = e.target.files[0]; if (f) handleFile(f, side); }

function handleFile(file, side) {
  const ext = file.name.split('.').pop().toLowerCase();
  if (!['pdf','docx'].includes(ext)) { toast('⚠ Только .pdf и .docx файлы'); return; }
  S.files[side] = {name:file.name, size:file.size, file};
  /* BACKEND:
     const fd = new FormData(); fd.append('file', file);
     fetch(`${CFG.API_BASE}/upload`, {method:'POST', body:fd})
       .then(r => r.json()).then(d => S.files[side].serverId = d.fileId); */
  const fp = document.getElementById('fp-' + side);
  const dz = document.getElementById('dz-' + side);
  fp.innerHTML = `
    <div class="fp-ico"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14,2 14,8 20,8"/></svg></div>
    <div><div class="fp-name">${file.name}</div><div class="fp-size">${(file.size/1024).toFixed(0)} KB</div></div>
    <button class="fp-rm" onclick="removeFile('${side}')"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>`;
  fp.classList.remove('hidden');
  dz.style.display = 'none';
}

function removeFile(side) {
  S.files[side] = null;
  document.getElementById('fp-' + side).classList.add('hidden');
  document.getElementById('dz-' + side).style.display = '';
  document.getElementById('fi-' + side).value = '';
}

function startCompare() {
  if (!S.files.old || !S.files.new) { toast('⚠ Загрузите оба файла для сравнения'); return; }
  document.getElementById('progress-wrap').classList.remove('hidden');
  document.getElementById('btn-compare').disabled = true;
  /* BACKEND: заменить simulateProgress на:
     fetch(`${CFG.API_BASE}/compare`, {method:'POST', headers:{'Content-Type':'application/json'},
       body: JSON.stringify({oldFileId: S.files.old.serverId, newFileId: S.files.new.serverId})})
       .then(r => r.json()).then(d => { S.jobId = d.jobId; pollJob(d.jobId); }); */
  simulateProgress(() => {
    showPage('compare-table');
    document.getElementById('progress-wrap').classList.add('hidden');
    document.getElementById('btn-compare').disabled = false;
  });
}

function simulateProgress(onDone) {
  const fill = document.getElementById('progress-fill');
  const steps = document.querySelectorAll('.ps');
  const labels = ['Парсинг документов...','Структурный анализ...','Семантический анализ...','Проверка законодательства РБ...','Формирование отчёта...'];
  let pct = 0, si = 0;
  const stepAt = [20, 40, 60, 80, 100];
  const iv = setInterval(() => {
    pct = Math.min(pct + 3, 100);
    fill.style.width = pct + '%';
    const ns = stepAt.findIndex(s => pct <= s);
    if (ns !== si && ns >= 0) {
      steps[si]?.classList.remove('active'); steps[si]?.classList.add('done');
      si = ns; steps[si]?.classList.add('active');
      document.getElementById('progress-label').textContent = labels[si] || '';
    }
    if (pct >= 100) { clearInterval(iv); setTimeout(onDone, 500); }
  }, 55);
}

/**
 * Polling статуса job
 * BACKEND: GET /api/jobs/:jobId/status
 * Ответ: { status: 'pending'|'processing'|'done'|'error', progress: 0-100, step: string, message?: string }
 */
function pollJob(jobId) {
  S.poll = setInterval(async () => {
    try {
      const r = await fetch(`${CFG.API_BASE}/jobs/${jobId}/status`);
      const d = await r.json();
      document.getElementById('progress-fill').style.width = d.progress + '%';
      document.getElementById('progress-label').textContent = d.step || '';
      if (d.status === 'done') { clearInterval(S.poll); showPage('compare-table'); }
      if (d.status === 'error') { clearInterval(S.poll); toast('Ошибка: ' + (d.message || 'попробуйте снова')); }
    } catch { clearInterval(S.poll); toast('Ошибка соединения с сервером'); }
  }, CFG.POLL_MS);
}

/* ── TABLE ── */
function renderTable(rows) {
  const tb = document.getElementById('ctbl-body');
  if (!tb) return;
  tb.innerHTML = rows.map(r => `
    <tr>
      <td>${r.n}</td>
      <td class="td-s">${r.s}</td>
      <td class="td-o"><span class="clip" title="${r.old}">${r.old}</span></td>
      <td class="td-n"><span class="clip" title="${r.nw}">${r.nw}</span></td>
      <td class="td-t">${r.type}</td>
      <td><span class="bdg ${rBdg(r.risk)}">${rFull(r.risk)}</span></td>
      <td class="td-m">${r.law}</td>
      <td class="td-r"><span class="clip" title="${r.rec}">${r.rec}</span></td>
      <td><button class="dots-btn" onclick="rowMenu(event,${r.n})">···</button></td>
    </tr>`).join('');
  const red = rows.filter(r => r.risk === 'red').length;
  const el = document.getElementById('tbl-summary');
  if (el) el.textContent = `${red} высоких приоритета · ${rows.length} изменений`;
}

function filterTable(q) { S.tbl.search = q.toLowerCase(); applyTblFilter(); }

function filterTableRisk(risk, btn) {
  S.tbl.risk = risk;
  const g = document.getElementById('tbl-risk-filter');
  if (g) { g.querySelectorAll('.fchip').forEach(c => c.classList.remove('active')); btn.classList.add('active'); }
  applyTblFilter();
}

function applyTblFilter() {
  /* BACKEND: GET /api/compare/:jobId/table?search=&risk= */
  const {risk, search} = S.tbl;
  let f = CHANGES;
  if (risk !== 'all') f = f.filter(r => r.risk === risk);
  if (search) f = f.filter(r => r.s.toLowerCase().includes(search) || r.old.toLowerCase().includes(search) || r.nw.toLowerCase().includes(search));
  renderTable(f);
}

function sortTable(col) {
  S.sort.dir = S.sort.col === col ? (S.sort.dir === 'asc' ? 'desc' : 'asc') : 'asc';
  S.sort.col = col;
  const ord = {red:0, orange:1, green:2};
  const s = [...CHANGES].sort((a, b) => {
    const va = col === 'risk' ? ord[a.risk] : a.type;
    const vb = col === 'risk' ? ord[b.risk] : b.type;
    return S.sort.dir === 'asc' ? (va > vb ? 1 : -1) : (va < vb ? 1 : -1);
  });
  renderTable(s);
}

function toggleTableDense() {
  document.getElementById('ctbl').classList.toggle('dense');
  toast('Вид таблицы изменён');
}

function rowMenu(e, n) { e.stopPropagation(); toast(`Строка ${n}: действия`); }
function tblPage(dir) { toast('Пагинация (требуется backend)'); }
function setPerPage(v) { S.perPage = v; /* BACKEND: перезапросить */ }

function switchTab(name, btn) {
  document.querySelectorAll('.tab').forEach(b => b.classList.remove('active')); btn.classList.add('active');
  document.querySelectorAll('.tab-pane').forEach(p => p.classList.add('hidden'));
  document.getElementById('tab-' + name).classList.remove('hidden');
}

/* ── VTV ── */
function renderVTV() {
  /* BACKEND: HTML с тегами vdel/vadd из GET /api/compare/:jobId/diff */
  document.getElementById('vtv-old').innerHTML = `
    <h4>Статья 218. Стандартные налоговые вычеты</h4>
    <p>4) налоговый вычет за каждый месяц налогового периода распространяется на родителя, супруга родителя, усыновителя,
    <span class="vdel" onclick="vtvClick(event,'red','П.2.4','Изменение обязанности','ст.55 ТК РБ','Проверить корректность обязанностей')">опекуна, попечителя, приёмного родителя, супруга приёмного родителя,</span>
    на обеспечении которых находится ребёнок, в следующих размерах:</p>
    <p><span class="vdel" onclick="vtvClick(event,'red','П.2.4','Изменение обязанности','ст.55 ТК РБ','Проверить корректность')">с 1 января по 31 декабря 2011 года включительно:</span></p>
    <p>1 <span class="vdel" onclick="vtvClick(event,'orange','П.4.1','Изменение срока','Закон №433-З','Проверить достаточность срока')">000</span> рублей на первого ребёнка;</p>
    <p>1 000 рублей на второго ребёнка;</p>
    <p>3 000 рублей на третьего и каждого последующего ребёнка;</p>
    <p><span class="vdel" onclick="vtvClick(event,'red','Раздел III','Удаление раздела','Закон №130-З','Проверить обязательность процедуры')">3 000 рублей на каждого ребёнка-инвалида до 18 лет</span></p>`;

  document.getElementById('vtv-new').innerHTML = `
    <h4>Статья 218. Стандартные налоговые вычеты</h4>
    <p>4) налоговый вычет за каждый месяц налогового периода распространяется на родителя, супруга родителя, усыновителя, на обеспечении которых находится ребёнок, в следующих размерах:</p>
    <p>1 <span class="vadd" onclick="vtvClick(event,'green','П.4.1','Изменение суммы','—','Повышает прозрачность')">400</span> рублей на первого ребёнка;</p>
    <p>1 400 рублей на второго ребёнка;</p>
    <p>3 000 рублей на третьего и каждого последующего ребёнка;</p>
    <p>12 000 рублей на каждого ребёнка-инвалида или учащегося очной формы до 24 лет, если инвалид I–II группы;</p>
    <p><span class="vadd" onclick="vtvClick(event,'green','Статья 12','Добавление субъекта','—','Проверить корректность распределения')">налоговый вычет распространяется на опекуна, попечителя, приёмного родителя на обеспечении которых находится ребёнок:</span></p>
    <p>1 400 рублей на первого ребёнка;</p>
    <p>1 400 рублей на второго ребёнка;</p>
    <p>6 000 рублей на каждого ребёнка-инвалида;</p>`;
}

function vtvClick(e, risk, sec, type, law, rec) {
  e.stopPropagation();
  const bar = document.getElementById('vtv-bar');
  document.getElementById('vb-sec').textContent  = sec;
  document.getElementById('vb-type').textContent = type;
  document.getElementById('vb-law').textContent  = law;
  document.getElementById('vb-rec').textContent  = rec;
  const bdg = document.getElementById('vb-bdg');
  bdg.className = 'bdg ' + rBdg(risk);
  bdg.textContent = rFull(risk);
  bar.classList.remove('hidden');
}

/* ── HIERARCHY & VIOLATIONS ── */
function renderHierarchy() {
  const el = document.getElementById('hier-list');
  if (!el) return;
  el.innerHTML = HIERARCHY.map(h => `
    <div class="hier-item">
      <span class="hi-num">${h.n}</span>
      <span class="hi-txt">${h.text}</span>
      <span class="hi-dot" style="background:${h.ok ? 'var(--grn)' : 'var(--org)'}"></span>
    </div>`).join('');
}

function renderViolations() {
  /* BACKEND: VIOLATIONS заменить на GET /api/compare/:jobId/violations */
  const el = document.getElementById('viol-list');
  if (!el) return;
  el.innerHTML = VIOLATIONS.map(v => `
    <div class="viol-card ${v.risk === 'orange' ? 'warn' : ''}">
      <div class="viol-top">
        <span class="viol-sec">${v.s}</span>
        <span class="bdg ${rBdg(v.risk)}">${rFull(v.risk)}</span>
      </div>
      <p class="viol-txt">${v.text}</p>
      <p class="viol-law"><a href="${v.href}" target="_blank" rel="noopener">${v.law} · pravo.by ↗</a></p>
    </div>`).join('');
}

/* ── CHAIN ── */
function renderChainTimeline() {
  const el = document.getElementById('chain-timeline');
  if (!el) return;
  el.innerHTML = CHAIN_VERSIONS.map(v => {
    const dc = {red:'ct-dot-r', orange:'ct-dot-o', green:'ct-dot-g'}[v.risk] || 'ct-dot-g';
    const w = (v.red + v.org + v.grn) || 1;
    const bars = [
      v.red  ? `<span class="ct-seg bs-r" style="width:${Math.round(v.red/w*60)}px"></span>` : '',
      v.org  ? `<span class="ct-seg bs-o" style="width:${Math.round(v.org/w*60)}px"></span>` : '',
      v.grn  ? `<span class="ct-seg bs-g" style="width:${Math.round(v.grn/w*60)}px"></span>` : '',
    ].join('');
    return `
    <div class="ct-item">
      <div class="ct-dot ${dc}"></div>
      <div class="ct-card">
        <div class="ct-head">
          <span class="ct-ver">${v.ver}</span>
          <span class="ct-date">${v.date}</span>
        </div>
        <div class="ct-title">${v.title}</div>
        <div class="ct-meta">
          <span class="ct-changes">${v.changes} изм.</span>
          <div class="ct-bars">${bars}</div>
        </div>
        <div class="ct-author">Автор: ${v.author}</div>
        <div class="ct-actions">
          <button class="ct-btn" onclick="showPage('compare-table')">Сравнить</button>
          <button class="ct-btn" onclick="exportDocx()">Отчёт</button>
        </div>
      </div>
    </div>`;
  }).join('');
}

function renderChainChart() {
  /* BACKEND: данные из /api/acts/:id/chain → chartData */
  const svg = document.getElementById('chain-svg');
  if (!svg) return;
  const W = 400, H = 180, PAD = {t:20, r:20, b:30, l:30};
  const cw = W - PAD.l - PAD.r;
  const ch = H - PAD.t - PAD.b;
  const data = CHAIN_VERSIONS.slice().reverse();
  const maxY = Math.max(...data.map(d => d.red + d.org + d.grn)) || 10;
  const xs = data.map((_, i) => PAD.l + (i / (data.length - 1)) * cw);

  const lineR = data.map((d,i) => `${xs[i]},${PAD.t + ch - (d.red / maxY) * ch}`).join(' ');
  const lineO = data.map((d,i) => `${xs[i]},${PAD.t + ch - (d.org / maxY) * ch}`).join(' ');
  const lineG = data.map((d,i) => `${xs[i]},${PAD.t + ch - (d.grn / maxY) * ch}`).join(' ');

  const labels = data.map((d,i) => `<text x="${xs[i]}" y="${H - 5}" text-anchor="middle" font-size="10" fill="#a1a1aa">${d.ver}</text>`).join('');
  const gridY = [0, .25, .5, .75, 1].map(f => {
    const y = PAD.t + ch * (1 - f);
    return `<line x1="${PAD.l}" y1="${y}" x2="${W - PAD.r}" y2="${y}" stroke="#e4e4e7" stroke-width="1"/>`;
  }).join('');

  svg.innerHTML = `
    ${gridY}
    <polyline points="${lineG}" fill="none" stroke="#22c55e" stroke-width="2" stroke-linejoin="round"/>
    <polyline points="${lineO}" fill="none" stroke="#f97316" stroke-width="2" stroke-linejoin="round"/>
    <polyline points="${lineR}" fill="none" stroke="#ef4444" stroke-width="2" stroke-linejoin="round"/>
    ${data.map((d,i) => `
      <circle cx="${xs[i]}" cy="${PAD.t + ch - (d.grn / maxY) * ch}" r="3.5" fill="#22c55e"/>
      <circle cx="${xs[i]}" cy="${PAD.t + ch - (d.org / maxY) * ch}" r="3.5" fill="#f97316"/>
      <circle cx="${xs[i]}" cy="${PAD.t + ch - (d.red / maxY) * ch}" r="3.5" fill="#ef4444"/>
    `).join('')}
    ${labels}`;
}

/* ── EXPORT & NTSI ── */
/** BACKEND: GET /api/compare/:jobId/export?format=docx
 *  Генерирует Word с:
 *  - Таблицей было/стало/статья/риск/рекомендация
 *  - Гиперссылками pravo.by для каждого НПА
 *  - Юридически корректными комментариями
 *  Content-Type: application/vnd.openxmlformats-officedocument.wordprocessingml.document
 */
function exportDocx() {
  /* window.location.href = `${CFG.API_BASE}/compare/${S.jobId}/export?format=docx`; */
  toast('📄 Экспорт .docx (требуется backend)');
}

/** BACKEND: открывает НЦПИ, опционально с query */
function openNtsi() { window.open('https://pravo.by', '_blank', 'noopener'); }

function openNtsiModal() {
  document.getElementById('modal-ntsi').classList.remove('hidden');
  setTimeout(() => document.getElementById('ntsi-q')?.focus(), 100);
}

function ntsiType(t, btn) {
  document.querySelectorAll('#modal-ntsi .fchip').forEach(c => c.classList.remove('active'));
  btn.classList.add('active');
}

function ntsiSearch() {
  const q = document.getElementById('ntsi-q')?.value.trim();
  if (!q) { toast('Введите поисковый запрос'); return; }
  /* BACKEND: GET /api/npa/search?query=<q> — прокси на pravo.by
     Ответ: { results: [{title, url, type, date}] }
     Показать в #ntsi-results */
  window.open(`https://pravo.by/search/?c=&text=${encodeURIComponent(q)}&q=&l=`, '_blank', 'noopener');
  closeModal('modal-ntsi');
}

/** BACKEND: POST /api/compare/:jobId/share → { shareUrl } */
function shareReport() {
  navigator.clipboard.writeText(window.location.href).then(() => toast('✓ Ссылка скопирована'));
}

/* ── MODALS ── */
function closeModal(id) { document.getElementById(id)?.classList.add('hidden'); }

function openShortcuts() { document.getElementById('modal-shortcuts').classList.remove('hidden'); }
function closeShortcuts() { closeModal('modal-shortcuts'); }

/* ── CTX MENU ── */
function showCtx(e, id) {
  e.stopPropagation();
  S.ctxId = id;
  const m = document.getElementById('ctx-menu');
  m.classList.remove('hidden');
  const x = Math.min(e.clientX, window.innerWidth - 170);
  const y = Math.min(e.clientY, window.innerHeight - 180);
  m.style.left = x + 'px'; m.style.top = y + 'px';
}
function closeCtx() { document.getElementById('ctx-menu').classList.add('hidden'); }
function ctxAction(action) {
  const id = S.ctxId;
  closeCtx();
  if (action === 'view')    openActDetailById(id);
  if (action === 'compare') showPage('upload');
  if (action === 'chain')   showPage('chain');
  if (action === 'export')  toast(`Экспорт акта #${id} (требуется backend)`);
  if (action === 'delete') {
    const a = ACTS.find(x => x.id === id);
    document.getElementById('modal-del-text').textContent = `Удалить «${a?.title}»? Это действие необратимо.`;
    document.getElementById('modal-del').classList.remove('hidden');
  }
}
function confirmDelete() {
  /* BACKEND: DELETE /api/acts/:id */
  closeModal('modal-del');
  toast('✓ Акт удалён (требуется backend)');
}

/* ── ASSISTANT ── */
function assResize(el) { el.style.height = 'auto'; el.style.height = Math.min(el.scrollHeight, 130) + 'px'; }
function assKey(e) { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); assSend(); } }
function sugSend(btn) { document.getElementById('ass-ta').value = btn.textContent; assSend(); }
function selectAct() { toast('Откройте нужный акт и вернитесь в ассистент'); }

function newChat() {
  /* BACKEND: POST /api/chats → { chatId } */
  document.getElementById('chat-msgs').innerHTML = '';
  document.getElementById('chat-msgs').classList.add('hidden');
  document.getElementById('ass-welcome').classList.remove('hidden');
}

/**
 * Отправка сообщения ассистенту
 * BACKEND: POST /api/chat
 * Body: { messages: [{role, content}], context: { actId?, jobId? } }
 * Ответ: SSE stream — data: {"token": "..."}\n\n
 * Парсинг: const es = new EventSource(url); es.onmessage = e => appendToken(e.data);
 */
function assSend() {
  const input = document.getElementById('ass-ta');
  const text  = input.value.trim();
  if (!text) return;
  document.getElementById('ass-welcome').classList.add('hidden');
  document.getElementById('chat-msgs').classList.remove('hidden');
  input.value = ''; input.style.height = 'auto';
  const msgs = document.getElementById('chat-msgs');
  msgs.innerHTML += `<div class="cm user"><div class="cm-av">Вы</div><div class="cm-bub">${esc(text)}</div></div>`;
  const tid = 'ty' + Date.now();
  msgs.innerHTML += `<div class="cm ai" id="${tid}"><div class="cm-av">AI</div><div class="cm-bub"><div class="typing"><span></span><span></span><span></span></div></div></div>`;
  msgs.scrollTop = msgs.scrollHeight;
  setTimeout(() => {
    document.getElementById(tid)?.remove();
    msgs.innerHTML += `<div class="cm ai"><div class="cm-av">AI</div><div class="cm-bub">${esc(getMockReply(text))}</div></div>`;
    msgs.scrollTop = msgs.scrollHeight;
  }, 900 + Math.random() * 500);
}

function getMockReply(t) {
  /* BACKEND: удалить getMockReply — использовать SSE-стрим от /api/chat */
  const l = t.toLowerCase();
  if (l.includes('п.2.4') || l.includes('объясни'))
    return 'П.2.4: замена «имеет право» → «обязан» меняет норму с диспозитивной на императивную. Согласно ч.1 ст.55 ТК РБ, это требует обоснования в пояснительной записке.';
  if (l.includes('риск'))
    return '3 критических риска:\n1. П.2.4 — нарушение ст.55 ТК РБ\n2. Раздел III — удаление обязательной процедуры (Закон №130-З)\n3. П.8.1 — нарушение ст.73 ТК РБ';
  if (l.includes('противоречи') || l.includes('тк'))
    return 'Противоречия с ТК РБ:\n• П.2.4 — ст.55 ТК РБ (нарушение прав работника)\n• П.8.1 — ст.73 ТК РБ (периодичность выплат не реже 2 раз/мес)';
  if (l.includes('заключени'))
    return 'Юридическое заключение:\n\nНовая редакция ПВТР ООО «ТехПром» содержит 3 нормы, противоречащих законодательству РБ. Акт не может быть введён в действие без устранения нарушений в пп. 2.4, 8.1 и восстановления Раздела III.\n\nОснования: ст.55, ст.73 ТК РБ; Закон №130-З.';
  if (l.includes('конституци'))
    return 'Соответствие Конституции РБ: нарушений не выявлено. Изменения находятся в рамках ст.41–43 Конституции.';
  if (l.includes('цепочк') || l.includes('редакци'))
    return 'Цепочка редакций v1→v5:\n• 27 изменений суммарно\n• Тенденция: ужесточение обязанностей работника\n• Наибольший риск: v5 (3 противоречия)\n• Рекомендую: откатить изменения п.2.4 и п.8.1';
  return `Анализирую запрос... Для точного ответа подключите Claude API с контекстом базы НПА Республики Беларусь.`;
}

/* ── ВЕРСИИ ── */
function versionMenu(e, num) { e.stopPropagation(); toast('Версия ' + num + ': контекст'); }

/* ── KEYBOARD ── */
function handleKbd(e) {
  const tag = document.activeElement?.tagName?.toLowerCase();
  if (['input','textarea','select'].includes(tag)) return;

  /* Закрытие модалок */
  if (e.key === 'Escape') {
    ['modal-del','modal-ntsi','modal-shortcuts'].forEach(id => closeModal(id));
    closeCtx();
    return;
  }

  /* Ctrl+K — поиск НЦПИ */
  if (e.ctrlKey && e.key === 'k') { e.preventDefault(); openNtsiModal(); return; }
  /* Ctrl+E — экспорт */
  if (e.ctrlKey && e.key === 'e') { e.preventDefault(); exportDocx(); return; }

  /* Одиночные клавиши */
  switch (e.key) {
    case '?': openShortcuts(); return;
    case 'n': case 'N': showPage('upload'); return;
    case 'v': case 'V': if (S.page === 'acts') setView(S.view === 'grid' ? 'list' : 'grid'); return;
    case 'd': case 'D': if (S.page === 'compare-table') toggleTableDense(); return;
    case '/': document.getElementById('acts-search')?.focus(); e.preventDefault(); return;
    case '1': if (S.page === 'compare-table') filterTableRisk('all', document.querySelector('#tbl-risk-filter .fchip')); return;
    case '2': if (S.page === 'compare-table') filterTableRisk('red',    document.querySelectorAll('#tbl-risk-filter .fchip')[1]); return;
    case '3': if (S.page === 'compare-table') filterTableRisk('orange', document.querySelectorAll('#tbl-risk-filter .fchip')[2]); return;
    case '4': if (S.page === 'compare-table') filterTableRisk('green',  document.querySelectorAll('#tbl-risk-filter .fchip')[3]); return;
  }

  /* G+X навигация */
  if (e.key === 'g' || e.key === 'G') { S.kbSeq = 'g'; clearTimeout(S.kbTimer); S.kbTimer = setTimeout(() => S.kbSeq = '', 1000); return; }
  if (S.kbSeq === 'g') {
    S.kbSeq = '';
    if (e.key === 'a' || e.key === 'A') showPage('acts');
    if (e.key === 'u' || e.key === 'U') showPage('upload');
    if (e.key === 'i' || e.key === 'I') showPage('assistant');
    if (e.key === 'c' || e.key === 'C') showPage('chain');
  }
}

/* ── TOAST ── */
function toast(msg) {
  const t = document.getElementById('toast');
  t.textContent = msg; t.classList.remove('hidden');
  clearTimeout(window._tt);
  window._tt = setTimeout(() => t.classList.add('hidden'), CFG.TOAST_MS);
}

/* ── UTILS ── */
function rBdg(r) { return {red:'bdg-r', orange:'bdg-o', green:'bdg-g'}[r] || 'bdg-g'; }
function rLbl(r) { return {red:'Проверить', orange:'Проверить', green:'Безопасно'}[r] || 'Безопасно'; }
function rFull(r){ return {red:'Противоречие', orange:'Проверить', green:'Безопасно'}[r] || 'Безопасно'; }
function barHtml(bar) { const m={r:'bs-r',o:'bs-o',g:'bs-g',e:'bs-e'}; return bar.map(c=>`<span class="bar-seg ${m[c]||'bs-e'}"></span>`).join(''); }
function pl(n,f1,f2,f5) { const a=Math.abs(n)%100,b=a%10; if(a>10&&a<20)return f5; if(b>1&&b<5)return f2; if(b===1)return f1; return f5; }
function esc(s) { return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/\n/g,'<br>').replace(/\*\*(.*?)\*\*/g,'<strong>$1</strong>'); }
