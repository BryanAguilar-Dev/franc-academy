// Franc Academy: interfaz. JavaScript sin frameworks.
//
// Cómo está organizado:
//   api      → habla con el servidor Go (/api/...)
//   state    → catálogo y progreso cargados en memoria
//   views    → una función por pantalla; cada una devuelve HTML
//   quiz     → el motor de preguntas que usan el test, el examen, el reto y el repaso
//   router   → lee la parte "#/..." de la URL y elige la pantalla
"use strict";

// ---------- utilidades ----------
const $ = (sel, root = document) => root.querySelector(sel);
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const norm = (s) => String(s ?? "").normalize("NFD").replace(/[̀-ͯ]/g, "").toLowerCase();
const pct = (a, b) => (b ? Math.round((a / b) * 100) : 0);
const shuffle = (arr) => { const a = arr.slice(); for (let i = a.length - 1; i > 0; i--) { const j = Math.floor(Math.random() * (i + 1)); [a[i], a[j]] = [a[j], a[i]]; } return a; };
const fmtDate = (iso) => new Date(iso).toLocaleDateString("es-ES", { day: "numeric", month: "long", year: "numeric" });
const fmtTime = (iso) => new Date(iso).toLocaleTimeString("es-ES", { hour: "2-digit", minute: "2-digit" });
const PASS = 0.7;

// ---------- api ----------
const api = {
  async get(path) {
    const r = await fetch(path, { headers: { Accept: "application/json" } });
    if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || `Error ${r.status}`);
    return r.json();
  },
  // results: [{ key, correct }] de cada pregunta, para el repaso espaciado.
  async event(type, ref, score = 0, total = 0, results = []) {
    const r = await fetch("/api/events", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ type, ref, score, total, results }),
    });
    if (!r.ok) throw new Error("No se pudo guardar el progreso");
    state.progress = await r.json();
  },
};

// ---------- estado ----------
const state = { catalog: null, progress: null, lessonCache: new Map() };
let mount = null; // función a ejecutar cuando el HTML de la pantalla ya está en la página

async function lesson(id) {
  if (!state.lessonCache.has(id)) state.lessonCache.set(id, await api.get(`/api/lessons/${encodeURIComponent(id)}`));
  return state.lessonCache.get(id);
}
const lessonsOf = (cat) => state.catalog.lessons.filter((l) => l.category === cat);
const categoryOf = (id) => state.catalog.categories.find((c) => c.id === id);
const lstate = (id) => state.progress.lessons[id] || null;
const isDone = (id) => Boolean(lstate(id)?.completedAt);

function status(id) {
  const s = lstate(id);
  if (s?.completedAt) return `<span class="badge done">Completada · ${s.bestScore}/${s.total}</span>`;
  if (s?.attempts) return `<span class="badge read">Test: ${s.bestScore}/${s.total}</span>`;
  if (s?.readAt) return `<span class="badge read">Leída</span>`;
  return `<span class="badge">Nueva</span>`;
}

// La lección recomendada: la primera sin completar, empezando por la categoría
// en la que estabas. Si lo has completado todo, la que peor nota tiene.
function recommendation() {
  const all = state.catalog.lessons;
  if (!all.length) return null;
  const last = state.progress.lastLesson && all.find((l) => l.id === state.progress.lastLesson);
  const pending = all.filter((l) => !isDone(l.id));
  if (pending.length) {
    const sameCat = last && pending.find((l) => l.category === last.category && l.id !== last.id);
    return { lesson: sameCat || pending.find((l) => l.id !== last?.id) || pending[0], reason: "Siguiente en tu ruta" };
  }
  const worst = all.slice().sort((a, b) => (lstate(a.id).bestScore / lstate(a.id).total) - (lstate(b.id).bestScore / lstate(b.id).total))[0];
  return { lesson: worst, reason: "Repaso: es la que peor nota tiene" };
}

// "Continuar": la última lección si la empezaste pero no la has completado.
function continuing() {
  const id = state.progress.lastLesson;
  if (!id || isDone(id)) return null;
  return state.catalog.lessons.find((l) => l.id === id) || null;
}

const ICONS = {
  universo: '<circle cx="12" cy="12" r="4"/><ellipse cx="12" cy="12" rx="10" ry="4" transform="rotate(-25 12 12)"/>',
  ciencia: '<path d="M9 3h6M10 3v6l-5 9a2 2 0 0 0 2 3h10a2 2 0 0 0 2-3l-5-9V3"/>',
  historia: '<path d="M4 21h16M5 21V10M19 21V10M9 21V10M15 21V10M3 10l9-6 9 6z"/>',
  mente: '<path d="M9 3a4 4 0 0 0-4 4 4 4 0 0 0-1 7 4 4 0 0 0 5 6V3zM15 3a4 4 0 0 1 4 4 4 4 0 0 1 1 7 4 4 0 0 1-5 6V3z"/>',
  tecnologia: '<rect x="6" y="6" width="12" height="12" rx="2"/><path d="M9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M2 15h4M18 9h4M18 15h4"/>',
  cultura: '<path d="M4 5a2 2 0 0 1 2-2h13v16H6a2 2 0 0 0-2 2zM4 19V5"/>',
  go: '<path d="M8 7l-5 5 5 5M16 7l5 5-5 5M14 4l-4 16"/>',
};
// Bloque de código: se escapa y se respeta tal cual (espacios y saltos de línea).
const codeBlock = (code) => `<pre class="code"><code>${esc(code)}</code></pre>`;

// «Copiar código» del ejercicio. Se escucha en todo el documento porque las
// vistas se pintan con innerHTML y el botón aún no existe al cargar la página.
document.addEventListener("click", async (e) => {
  const btn = e.target.closest("[data-copy]");
  if (!btn) return;
  const code = btn.closest(".practice")?.querySelector("pre code")?.textContent ?? "";
  try {
    await navigator.clipboard.writeText(code);
    btn.textContent = "Copiado ✓";
  } catch {
    btn.textContent = "No se pudo copiar";
  }
  setTimeout(() => { btn.textContent = "Copiar código"; }, 2000);
});

const icon = (id) => `<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[id] || ""}</svg>`;

// ---------- pantallas ----------
const views = {};

views.home = async () => {
  const all = state.catalog.lessons;
  const done = all.filter((l) => isDone(l.id)).length;
  const rec = recommendation();
  const cont = continuing();
  const today = new Date().toLocaleDateString("sv-SE", { timeZone: "Europe/Madrid" });
  const dailyDone = state.progress.daily[today];
  const review = await api.get("/api/review").catch(() => ({ pending: 0 })); // si falla, el inicio sigue funcionando

  const recCard = rec ? `
    <article class="card highlight">
      <p class="eyebrow">Lección para hoy · ${esc(rec.reason)}</p>
      <h2>${esc(rec.lesson.title)}</h2>
      <p class="muted">${esc(rec.lesson.intro)}</p>
      <div class="row"><a class="btn" href="#/l/${esc(rec.lesson.id)}">Empezar · ${rec.lesson.minutes} min</a>
      <span class="small muted">${esc(categoryOf(rec.lesson.category).title)}</span></div>
    </article>` : "";

  const contCard = cont && cont.id !== rec?.lesson.id ? `
    <article class="card">
      <p class="eyebrow">Continuar aprendiendo</p>
      <h3>${esc(cont.title)}</h3>
      <div class="row">${status(cont.id)}</div>
      <div class="row">
        <a class="btn ghost" href="#/l/${esc(cont.id)}">Volver a la lección</a>
        <a class="btn" href="#/t/${esc(cont.id)}">Hacer el test</a>
      </div>
    </article>` : "";

  // Solo aparece si hoy toca repasar algo.
  const reviewCard = review.pending ? `
    <article class="card">
      <p class="eyebrow">Repaso</p>
      <h3>${review.pending === 1 ? "1 pregunta para repasar hoy" : `${review.pending} preguntas para repasar hoy`}</h3>
      <p class="muted small">Preguntas que fallaste. Cuantas más veces las aciertes, más tardarán en volver.</p>
      <div class="row"><a class="btn" href="#/repaso">Repasar</a></div>
    </article>` : "";

  const dailyCard = `
    <article class="card">
      <p class="eyebrow">Reto diario</p>
      <h3>${dailyDone ? `Hecho: ${dailyDone.score}/${dailyDone.total}` : "5 preguntas para repasar"}</h3>
      <p class="muted small">${dailyDone ? "Mañana tendrás preguntas nuevas." : "Preguntas de lo que ya has estudiado. Cambia cada día."}</p>
      <div class="row"><a class="btn ${dailyDone ? "ghost" : ""}" href="#/reto">${dailyDone ? "Ver el reto" : "Hacer el reto"}</a></div>
    </article>`;

  const cats = state.catalog.categories.map((c) => {
    const ls = lessonsOf(c.id);
    const d = ls.filter((l) => isDone(l.id)).length;
    return `
      <a class="card cat ${ls.length ? "" : "empty"}" href="#/c/${esc(c.id)}">
        <div class="spread"><span class="icon">${icon(c.icon)}</span>
          <span class="small muted">${ls.length ? `${d}/${ls.length} lecciones` : "Próximamente"}</span></div>
        <h3>${esc(c.title)}</h3>
        <p class="small muted">${esc(c.description)}</p>
        ${ls.length ? `<div class="bar" role="progressbar" aria-label="Progreso en ${esc(c.title)}" aria-valuenow="${pct(d, ls.length)}" aria-valuemin="0" aria-valuemax="100"><span style="width:${pct(d, ls.length)}%"></span></div>` : ""}
      </a>`;
  }).join("");

  return `
    <h1>Hola, ¿qué aprendemos hoy?</h1>
    <div class="grid">${recCard}${reviewCard}${contCard}${dailyCard}</div>
    <section class="card" aria-labelledby="prog-h">
      <h2 id="prog-h">Tu progreso</h2>
      <div class="stats">
        <div class="stat"><b>${done}</b><span>de ${all.length} lecciones completadas</span></div>
        <div class="stat"><b>${Object.keys(state.progress.exams).length}</b><span>exámenes hechos</span></div>
        <div class="stat"><b>${Object.keys(state.progress.daily).length}</b><span>retos diarios</span></div>
      </div>
      <div class="bar" role="progressbar" aria-label="Progreso total" aria-valuenow="${pct(done, all.length)}" aria-valuemin="0" aria-valuemax="100"><span style="width:${pct(done, all.length)}%"></span></div>
    </section>
    <section aria-labelledby="cats-h" style="display:flex;flex-direction:column;gap:16px">
      <div class="spread"><h2 id="cats-h">Categorías</h2><a href="#/buscar">Buscar una lección</a></div>
      <div class="grid">${cats}</div>
    </section>`;
};

views.category = (id) => {
  const c = categoryOf(id);
  if (!c) return notFound();
  const ls = lessonsOf(id);
  const d = ls.filter((l) => isDone(l.id)).length;
  const exam = state.progress.exams[id];
  const list = ls.map((l) => `
    <li><a href="#/l/${esc(l.id)}">
      <span class="num">${l.order}</span>
      <span class="grow"><strong>${esc(l.title)}</strong><br><span class="small muted">${l.minutes} min</span></span>
      ${status(l.id)}
    </a></li>`).join("");
  return `
    <div class="narrow">
      <p><a href="#/">← Inicio</a></p>
      <div class="row"><span class="cat"><span class="icon">${icon(c.icon)}</span></span><h1>${esc(c.title)}</h1></div>
      <p class="muted">${esc(c.description)}</p>
      ${ls.length ? `
        <div class="card">
          <div class="spread"><strong>${d} de ${ls.length} lecciones completadas</strong><span class="small muted">${pct(d, ls.length)}%</span></div>
          <div class="bar"><span style="width:${pct(d, ls.length)}%"></span></div>
        </div>
        <ol class="lessons">${list}</ol>
        <div class="card">
          <h2>Examen de la categoría</h2>
          <p class="muted small">Hasta 20 preguntas de todas las lecciones de ${esc(c.title)}, en orden aleatorio.
          ${d < ls.length ? "Te recomendamos hacerlo cuando completes todas las lecciones." : ""}</p>
          ${exam ? `<p>Mejor nota: <strong>${exam.score}/${exam.total}</strong> (${fmtDate(exam.at)})</p>` : ""}
          <div class="row"><a class="btn ${d < ls.length ? "ghost" : ""}" href="#/e/${esc(id)}">Hacer el examen</a></div>
        </div>` : `<div class="card"><p>Las lecciones de esta categoría están en preparación. Se irán publicando por tandas, revisadas y con sus fuentes.</p></div>`}
    </div>`;
};

views.lesson = async (id) => {
  const l = await lesson(id);
  const c = categoryOf(l.category);
  const ls = lessonsOf(l.category);
  const idx = ls.findIndex((x) => x.id === id);
  const next = ls[idx + 1];
  if (!lstate(id)?.readAt) api.event("read", id).catch(() => {});
  // `así` dentro de un párrafo se muestra como código (se usa en las lecciones de Go).
  const para = (ps) => ps.map((p) => `<p>${esc(p).replace(/`([^`]+)`/g, "<code>$1</code>")}</p>`).join("");
  return `
    <article class="narrow lesson">
      <p class="ui"><a href="#/c/${esc(c.id)}">← ${esc(c.title)}</a></p>
      <header style="display:flex;flex-direction:column;gap:10px">
        <p class="eyebrow">Lección ${l.order} · ${l.minutes} min de lectura</p>
        <h1>${esc(l.title)}</h1>
        <p class="intro">${esc(l.intro)}</p>
      </header>
      ${l.illustration ? `<figure><img src="/media/${esc(l.category)}/${esc(l.illustration.file)}" alt="${esc(l.illustration.alt)}" loading="lazy"><figcaption>${esc(l.illustration.caption)}</figcaption></figure>` : ""}
      ${l.sections.map((s) => `
        <section><h2>${esc(s.heading)}</h2>${para(s.body)}${s.example ? `<p class="example">${esc(s.example)}</p>` : ""}${s.code ? codeBlock(s.code) : ""}</section>`).join("")}
      <aside class="curious"><h2>Curiosidades</h2><ul>${l.curiosities.map((x) => `<li>${esc(x)}</li>`).join("")}</ul></aside>
      <section><h2>Conceptos clave</h2>
        <dl class="concepts">${l.keyConcepts.map((k) => `<div><dt>${esc(k.term)}</dt><dd>${esc(k.definition)}</dd></div>`).join("")}</dl></section>
      <section class="summary"><h2>Resumen</h2><ul>${l.summary.map((x) => `<li>${esc(x)}</li>`).join("")}</ul></section>
      <section><h2>Fuentes</h2>
        <ul class="sources">${l.sources.map((s) => `<li><a href="${esc(s.url)}" target="_blank" rel="noopener noreferrer">${esc(s.title)}</a> · ${esc(s.publisher)}</li>`).join("")}</ul>
        <p class="small muted ui">Texto original de Franc Academy, redactado a partir de estas fuentes.</p></section>
      ${l.practice ? `<section class="card practice"><h2>Practica</h2>${para([l.practice.task])}${codeBlock(l.practice.code)}
        <div class="row ui"><button class="btn" type="button" data-copy>Copiar código</button>
        <a class="btn ghost" href="https://go.dev/play/" target="_blank" rel="noopener noreferrer">Abrir Go Playground</a></div>
        <p class="small muted ui">Copia el código, pégalo en el Playground y pulsa Run.</p></section>` : ""}
      <div class="card ui">
        <h2>¿Lo has entendido?</h2>
        <p class="muted">10 preguntas para comprobarlo. Con 7 aciertos la lección queda completada.</p>
        <div class="row"><a class="btn" href="#/t/${esc(id)}">Hacer el test</a>
        ${next ? `<a class="btn ghost" href="#/l/${esc(next.id)}">Siguiente lección</a>` : ""}</div>
      </div>
    </article>`;
};

views.test = async (id) => {
  const l = await lesson(id);
  return quiz({
    title: `Test: ${l.title}`,
    back: { href: `#/l/${id}`, label: "Volver a la lección" },
    questions: l.questions,
    onFinish: (score, total, results) => api.event("quiz", id, score, total, results),
    finishText: (score, total) => {
      const passed = score >= PASS * total;
      return passed ? "Lección completada. Bien hecho." : `Necesitas ${Math.ceil(PASS * total)} aciertos para completarla. Repasa los fallos y vuelve a intentarlo cuando quieras.`;
    },
  });
};

views.exam = async (cat) => {
  const c = categoryOf(cat);
  if (!c) return notFound();
  const data = await api.get(`/api/exam/${encodeURIComponent(cat)}`);
  return quiz({
    title: `Examen: ${c.title}`,
    back: { href: `#/c/${cat}`, label: `Volver a ${c.title}` },
    questions: data.questions,
    onFinish: (score, total, results) => api.event("exam", cat, score, total, results),
    finishText: (score, total) => (score >= PASS * total ? "Dominas esta categoría." : "Repasa las lecciones de las preguntas falladas."),
  });
};

views.daily = async () => {
  const data = await api.get("/api/daily");
  if (data.done) {
    return `<div class="narrow"><h1>Reto diario</h1>
      <div class="card"><p class="eyebrow">${fmtDate(data.done.at)}</p>
      <p class="score">${data.done.score}/${data.done.total}</p>
      <p class="muted">Ya has hecho el reto de hoy. Mañana habrá preguntas nuevas.</p>
      <div class="row"><a class="btn" href="#/">Volver al inicio</a></div></div></div>`;
  }
  return quiz({
    title: "Reto diario",
    intro: "Cinco preguntas, preferiblemente de lecciones que ya has leído. Cuenta el primer intento del día.",
    back: { href: "#/", label: "Inicio" },
    questions: data.questions,
    onFinish: (score, total, results) => api.event("daily", data.date, score, total, results),
    finishText: () => "Reto completado. Vuelve mañana.",
  });
};

views.review = async () => {
  const data = await api.get("/api/review");
  if (!data.questions.length) {
    return `<div class="narrow"><h1>Repaso</h1>
      <div class="card"><p>Hoy no tienes nada que repasar.</p>
      <p class="muted small">Cuando falles una pregunta en un test, un examen o el reto, aparecerá aquí al día siguiente.</p>
      <div class="row"><a class="btn" href="#/">Volver al inicio</a></div></div></div>`;
  }
  const left = data.pending - data.questions.length;
  return quiz({
    title: "Repaso",
    intro: "Preguntas que fallaste. Si la aciertas, volverá dentro de más días; si la fallas, mañana otra vez.",
    back: { href: "#/", label: "Inicio" },
    questions: data.questions,
    onFinish: (score, total, results) => api.event("review", data.date, score, total, results),
    finishText: () => (left > 0 ? `Quedan ${left} más para hoy. Puedes seguir desde el inicio.` : "Repaso de hoy hecho."),
  });
};

views.history = () => {
  const label = { read: "Leíste", quiz: "Test", exam: "Examen", daily: "Reto diario", review: "Repaso" };
  const name = (e) => {
    if (e.type === "read" || e.type === "quiz") return state.catalog.lessons.find((l) => l.id === e.ref)?.title || e.ref;
    if (e.type === "exam") return categoryOf(e.ref)?.title || e.ref;
    return "";
  };
  const ev = state.progress.history.slice().reverse();
  if (!ev.length) return `<div class="narrow"><h1>Historial</h1><p class="muted">Aún no hay actividad. Empieza por una lección desde el <a href="#/">inicio</a>.</p></div>`;
  const groups = new Map();
  for (const e of ev) {
    const d = fmtDate(e.at);
    if (!groups.has(d)) groups.set(d, []);
    groups.get(d).push(e);
  }
  return `<div class="narrow"><h1>Historial de aprendizaje</h1>
    ${[...groups].map(([d, es]) => `<section style="display:flex;flex-direction:column;gap:8px"><h2>${esc(d)}</h2><ul class="history">
      ${es.map((e) => `<li><span><strong>${label[e.type]}</strong> ${esc(name(e))}</span>
        <span class="muted small">${e.type === "read" ? "" : `${e.score}/${e.total} · `}${fmtTime(e.at)}</span></li>`).join("")}
    </ul></section>`).join("")}</div>`;
};

views.search = (q = "") => `
  <div class="narrow">
    <h1>Buscar lecciones</h1>
    <label for="q" class="muted small">Busca por título, tema o concepto</label>
    <input id="q" type="search" value="${esc(q)}" placeholder="Por ejemplo: año luz, supernova, Big Bang" autocomplete="off">
    <div id="results" aria-live="polite"></div>
  </div>`;

function renderResults(q) {
  const box = $("#results");
  const words = norm(q).split(/\s+/).filter(Boolean);
  if (!words.length) { box.innerHTML = `<p class="muted">Hay ${state.catalog.lessons.length} lecciones disponibles.</p>`; return; }
  const hits = state.catalog.lessons
    .map((l) => {
      const title = norm(l.title), body = norm(`${l.intro} ${(l.concepts || []).join(" ")}`);
      let score = 0;
      for (const w of words) { if (title.includes(w)) score += 3; else if (body.includes(w)) score += 1; else return null; }
      return { l, score };
    })
    .filter(Boolean).sort((a, b) => b.score - a.score);
  box.innerHTML = hits.length
    ? `<ol class="lessons">${hits.map(({ l }) => `<li><a href="#/l/${esc(l.id)}"><span class="grow"><strong>${esc(l.title)}</strong><br>
        <span class="small muted">${esc(categoryOf(l.category).title)} · ${l.minutes} min</span></span>${status(l.id)}</a></li>`).join("")}</ol>`
    : `<p class="muted">No hay lecciones que coincidan con «${esc(q)}».</p>`;
}

const notFound = () => `<div class="narrow"><h1>No encontrado</h1><p><a href="#/">Volver al inicio</a></p></div>`;

// ---------- motor de preguntas ----------
// Muestra las preguntas de una en una. Al responder explica por qué la opción
// elegida es correcta o no, y cuál era la buena. Al final, nota y repaso de fallos.
function quiz({ title, intro, back, questions, onFinish, finishText }) {
  const qs = questions.map((q) => {
    const order = shuffle(q.options.map((_, i) => i)); // mezclar evita memorizar posiciones
    return { ...q, order };
  });
  let i = 0, score = 0;
  const wrong = [];
  const results = []; // { key, correct } por pregunta: lo usa el repaso espaciado

  mount = show;
  return `<div class="narrow" id="quiz"></div>`;

  function show() {
    const root = $("#quiz");
    if (!root) return;
    if (i >= qs.length) return finish(root);
    const q = qs[i];
    root.innerHTML = `
      <p><a href="${esc(back.href)}">← ${esc(back.label)}</a></p>
      <div class="spread"><h1 style="font-size:1.4rem">${esc(title)}</h1><span class="muted">${i + 1} / ${qs.length}</span></div>
      ${i === 0 && intro ? `<p class="muted">${esc(intro)}</p>` : ""}
      <div class="bar"><span style="width:${pct(i, qs.length)}%"></span></div>
      ${q.lessonTitle ? `<p class="eyebrow">${esc(q.lessonTitle)}</p>` : ""}
      <p class="question" id="qtext">${esc(q.q)}</p>
      <ul class="options" role="list" aria-labelledby="qtext">
        ${q.order.map((oi, k) => `<li><button class="option" data-i="${oi}"><span class="mark">${"ABCD"[k]}.</span> ${esc(q.options[oi].text)}</button></li>`).join("")}
      </ul>
      <p class="feedback" aria-live="assertive"></p>
      <div class="row" id="next"></div>`;
    root.querySelectorAll(".option").forEach((b) => b.addEventListener("click", () => answer(Number(b.dataset.i))));
    root.querySelector(".option").focus({ preventScroll: true });
    window.scrollTo(0, 0);
  }

  function answer(chosen) {
    const q = qs[i];
    const ok = chosen === q.answer;
    if (ok) score++; else wrong.push({ q, chosen });
    if (q.key) results.push({ key: q.key, correct: ok });
    document.querySelectorAll(".option").forEach((b) => {
      const oi = Number(b.dataset.i);
      b.disabled = true;
      if (oi === q.answer) b.classList.add("correct");
      if (oi === chosen && !ok) b.classList.add("wrong");
      if (oi === q.answer || oi === chosen) b.insertAdjacentHTML("beforeend", `<span class="why">${esc(q.options[oi].why)}</span>`);
    });
    const fb = $(".feedback");
    fb.className = `feedback ${ok ? "ok" : "ko"}`;
    fb.textContent = ok ? "Correcto." : "No es correcta. Mira la explicación de la respuesta buena.";
    const last = i === qs.length - 1;
    $("#next").innerHTML = `<button class="btn">${last ? "Ver resultado" : "Siguiente pregunta"}</button>`;
    const btn = $("#next button");
    btn.addEventListener("click", () => { i++; show(); });
    btn.focus({ preventScroll: true });
  }

  async function finish(root) {
    let saved = "";
    try { await onFinish(score, qs.length, results); } catch (e) { saved = `<p class="feedback ko">${esc(e.message)}. Tu resultado no se ha guardado.</p>`; }
    root.innerHTML = `
      <h1>${esc(title)}</h1>
      <div class="card">
        <p class="score">${score}/${qs.length}</p>
        <p>${esc(finishText(score, qs.length))}</p>${saved}
      </div>
      ${wrong.length ? `<section style="display:flex;flex-direction:column;gap:12px"><h2>Repasa tus fallos</h2>
        ${wrong.map(({ q, chosen }) => `<div class="card">
          ${q.lessonTitle ? `<p class="eyebrow">${esc(q.lessonTitle)}</p>` : ""}
          <p><strong>${esc(q.q)}</strong></p>
          <p class="feedback ko">Respondiste: ${esc(q.options[chosen].text)}</p>
          <p class="feedback ok">Correcta: ${esc(q.options[q.answer].text)}</p>
          <p>${esc(q.options[q.answer].why)}</p>
          ${q.lessonId ? `<p><a href="#/l/${esc(q.lessonId)}">Repasar la lección</a></p>` : ""}
        </div>`).join("")}</section>` : ""}
      <div class="row"><a class="btn" href="${esc(back.href)}">${esc(back.label)}</a><a class="btn ghost" href="#/">Inicio</a></div>`;
    $("h1", root).setAttribute("tabindex", "-1");
    $("h1", root).focus();
  }
}

// ---------- router ----------
const routes = [
  [/^#\/?$/, () => views.home(), "#/"],
  [/^#\/c\/([\w-]+)$/, (m) => views.category(m[1])],
  [/^#\/l\/([\w-]+)$/, (m) => views.lesson(m[1])],
  [/^#\/t\/([\w-]+)$/, (m) => views.test(m[1])],
  [/^#\/e\/([\w-]+)$/, (m) => views.exam(m[1])],
  [/^#\/reto$/, () => views.daily(), "#/reto"],
  [/^#\/repaso$/, () => views.review()],
  [/^#\/historial$/, () => views.history(), "#/historial"],
  [/^#\/buscar(?:\?q=(.*))?$/, (m) => views.search(decodeURIComponent(m[1] || "")), "#/buscar"],
];

async function render() {
  const hash = location.hash || "#/";
  const main = $("#main");
  let nav = null;
  try {
    // Recargar el progreso en cada pantalla: así ves lo que hiciste en otro dispositivo.
    state.progress = await api.get("/api/progress");
    mount = null;
    let html = notFound();
    for (const [re, fn, n] of routes) {
      const m = hash.match(re);
      if (m) { html = await fn(m); nav = n || null; break; }
    }
    main.innerHTML = html;
    if (mount) { mount(); mount = null; }
  } catch (e) {
    main.innerHTML = `<div class="narrow"><h1>Algo ha fallado</h1><p>${esc(e.message)}</p><p><a href="#/">Volver al inicio</a></p></div>`;
  }
  document.querySelectorAll(".topnav a").forEach((a) => a.removeAttribute("aria-current"));
  if (nav) document.querySelector(`.topnav a[href="${nav}"]`)?.setAttribute("aria-current", "page");
  const h1 = $("h1", main);
  document.title = h1 ? `${h1.textContent} · Franc Academy` : "Franc Academy";
  if (!$("#quiz")) { window.scrollTo(0, 0); main.focus({ preventScroll: true }); }

  const input = $("#q");
  if (input) {
    renderResults(input.value);
    input.addEventListener("input", () => { renderResults(input.value); history.replaceState(null, "", `#/buscar?q=${encodeURIComponent(input.value)}`); });
    input.focus();
  }
}

async function start() {
  try {
    [state.catalog, state.progress] = await Promise.all([api.get("/api/catalog"), api.get("/api/progress")]);
  } catch (e) {
    $("#main").innerHTML = `<div class="narrow"><h1>No se puede conectar con Franc</h1><p class="muted">${esc(e.message)}. Comprueba que estás conectado a Tailscale y que el servicio está en marcha.</p></div>`;
    return;
  }
  window.addEventListener("hashchange", render);
  render();
}
start();
