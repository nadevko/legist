---
rfc-id: 1
start-date: 2026-03-17
author: Baradzin Aliaksiei
co-authors:
  [
    "Naurasiuk Matvei",
    "Sarachuk Daniil",
    "Lisovitskiy Bogdan"
  ]
status: accepted
related-issues: []
---

# Summary

This RFC documents the decision to name the project **Легист** (Latin
transliteration: `legist`), with the primary domain **легист.бел**, and explains
why this name was chosen over the alternatives discussed during the team's
naming session on 2026-03-16/17.

# Motivation

- A project needs a name. Not just internally — judges at the hackathon,
  potential users, and future contributors form their first impression from the
  name alone.
- The name must work for the actual target audience: **legal professionals**,
  not general consumers or developers.
- The name should fit naturally into the existing landscape of CIS legal
  information systems while signaling something fresh and purposeful.
- A strong domain compounds the name's impact: `легист.бел` ties the product to
  Belarus and makes the name immediately legible as a domain-native brand.

# Detailed Design

## Chosen Name: Легист

**Легист** (Russian/Belarusian: _legist_; English Latin: `legist`) is a
historical and professional term for a specialist in written law — specifically,
one who interprets and applies codified statutes. The word derives from Latin
_legista_ (from _lex/legis_, "law").

### Why This Name Works

**Semantic fit.** The product is a tool for tracking, comparing, and navigating
legislative changes. A _legist_ is exactly the kind of professional who uses
such a tool. The name reflects the domain without describing a narrow feature.

**Audience alignment.** The target users — lawyers, legal analysts, compliance
officers — know this word. It does not need to be explained to them. For anyone
else, a quick search confirms its meaning and immediately elevates the perceived
professionalism of the product.

**Naming tradition.** The name fits the established naming model of serious CIS
legal systems: Эталон, Гарант, Кодекс, Әділет. These names imply authority and
precision without spelling out features. Легист belongs in this company.

**Brevity and phonetics.** Two syllables in Russian/Belarusian. Clean consonant
cluster. No ambiguous pronunciation. Memorable on first hearing.

**Domain availability.** The domain `легист.бел` was available at the time of
the decision at a cost of 42.5 BYN/year — an unremarkable sum for a
production-ready Belarusian-market domain. The `.бел` ccTLD reinforces the
product's national focus and makes the full string `легист.бел` read as a
coherent brand unit, not just a URL.

**Code-safe transliteration.** The Latin form `legist` is unambiguous, already
an English word, and requires no special encoding in codebases, CI configs, or
repository names.

# Examples and Interactions

| Context                     | Value             |
| --------------------------- | ----------------- |
| Product name                | Легист            |
| Primary domain              | легист.бел        |
| Code / repo identifier      | `legist`          |
| Premium tier (hypothetical) | Легист Энтерпрайз |
| iOS/Android app identifier  | `by.legist`       |

A repository previously named `lawdiff` was renamed to `legist` immediately
after the naming decision was finalized.

# Drawbacks

- **Not self-describing.** Unlike "ЮрСпутник" or "Право.бай", the name does not
  immediately communicate what the product does. A user unfamiliar with the term
  must look it up once.
- **Niche vocabulary.** "Легист" is a professional/historical term, not everyday
  speech. Outside of legal circles it may require brief explanation.
- **No English-language SEO value.** The Cyrillic name and `.бел` domain are
  optimized for the Belarusian/Russian-language market only. International
  expansion would require a separate brand decision.

# Alternatives

## `lawdiff`

Proposed as a repository name early in the discussion ("I totalitarianly decided
for all of you via the repo name"). Descriptive and developer-friendly, but:

- Purely English in a product aimed at a Russian/Belarusian-speaking legal
  market.
- Describes the implementation mechanism ("diff"), not the value delivered to
  the user.
- "lawdiff enterprise" was jokingly proposed as a premium tier, which reflects
  how toy-like the name felt to the team.

**Rejected:** wrong language for the target market (see Language Policy above),
wrong abstraction level — describes the implementation, not the value.

## `БелЮрКонтроль`

Proposed as a deliberately bureaucratic name in the Soviet institutional style
(cf. Роскомнадзор, Белтелеком). Generated immediate ironic enthusiasm precisely
because it sounds like a state regulator, not a product. The team noted that
this style ("БелКонсультантПлюс -\_-") describes the competition, not the
challenger.

**Rejected:** too institutional, no differentiation from existing market
players.

## `ЮрСпутник`

Received genuine positive discussion. Arguments in its favor: immediately
communicates "legal companion", fits the Soviet-retro-tech aesthetic, logo
potential (satellite), "народное" (accessible) quality.

Arguments against (summarized from discussion):

- "Спутник" means companion/satellite — a passive presence, not an active tool.
  It accompanies the user but doesn't act. "Консультант" helps without imposing;
  "Спутник" just floats alongside.
- "Юр-" is a truncated prefix that puts the legal aspect in a diminished first
  syllable. The legal dimension should lead, not be abbreviated.
- The Soviet-retro aesthetic is a stylistic gamble that could read as dated
  rather than nostalgic.
- "ЮрСпутник" in the existing CIS naming landscape (see Prior Art) would slot
  into the mid-tier functional-name category, not the authority-name category
  the product should aim for.

**Rejected:** passive semantics, weaker authority signal than Легист.

## `Легист AI` / `Легист.ai`

Briefly floated once "Легист" was accepted. Shot down on internal consistency
grounds: if the team committed to Cyrillic branding, mixing in English "AI" is
incoherent. The team also noted that "AI" as a suffix is a passing trend ("мода
из комода" — fashion from a chest-of-drawers).

**Rejected:** brand inconsistency, trend-chasing.

## `легист.бу` / `легист.бай` / `легист.бел`

Three domain variants were considered for the `.б*` Belarusian-market TLD space:

- `.бу` — not a real registered Belarusian ccTLD; dismissed immediately.
- `.бай` — Belarusian transliteration of `.by`; creative but unofficial.
- `.бел` — the official Belarusian Cyrillic IDN ccTLD. Pairs perfectly with a
  Cyrillic brand name, reinforces national market focus, and was available.

**Decision:** `легист.бел` selected unanimously once the availability and price
(42.5 BYN/year) were confirmed.

# Prior Art

## Naming Conventions in CIS Legal Information Systems

Legal information systems across the CIS follow a recognizable naming logic.
Names cluster around two archetypes:

**The law itself** — names that invoke the substance, authority, or ideal of law
as a concept: Кодекс (the code), Эталон (the standard), Әділет (justice). These
names claim to _be_ the law, or at least its definitive expression.

**A role in the legal process** — names that describe a participant or function
adjacent to legal work: Гарант (the guarantor — one who vouches), Консультант
(one who advises without deciding), Спутник (a companion who accompanies). These
names position the product as a helper.

Notably, the lawyer themselves never appears in these names. Naming a product
"Юрист" would position it either as a replacement for the professional or as a
tool for laypeople — both readings alienate the actual target audience. A
practicing lawyer does not want a tool that carries their own job title: it
either competes with them or signals it was built for someone else. So the
industry converges on adjacent roles — helpers, companions, guarantors — that
assist the professional without encroaching on their identity.

Легист sidesteps this entirely by using a historical term. No one today is
called a _legist_ in professional practice — it is an archaism rooted in the
Latin legal tradition, referring to a specialist in codified written law. This
makes it psychologically neutral: it does not compete with the user's
self-image, nor does it talk down to them. Instead it invokes the lineage of the
profession — the long tradition of those who work with statute — and flatters
the user by placing them in it.

English-language names (lawdiff, Skylex, Juro) exist in this space but are
concentrated in the LegalTech/startup segment aimed at developers or
international markets. For a Belarusian-market product targeting legal
professionals, a Russified or native-language name is the default expectation —
it signals seriousness, not novelty.

## Naming Session

The name was chosen during a team discussion on 2026-03-16/17. Earlier in the
session the team surveyed existing systems and identified three broader market
positioning models: authority-tier names (Эталон, Гарант, Кодекс),
functional/descriptive names (Право.бай, Нормативка.by), and LegalTech names
(Pravoved, Doczilla, Zakon.kz). The goal was to target the first tier.

# Unresolved Questions

- Domain purchase timing: the domain `легист.бел` had not been purchased as of
  the discussion. It was being monitored. A final decision on purchase was
  deferred until the MVP demonstrated viability ("if everything works on Friday,
  we'll think about it").
- Long-term internationalization strategy: if the product expands beyond
  Belarus, a parallel Latin-script brand identity will be needed. `legist.law`
  or similar was not discussed.

# Future Work

- Evaluate purchasing `легист.бел` post-hackathon if the product proceeds.
- Design a logotype consistent with the authority-tier naming tradition.
- Define the tier naming convention if a freemium model is pursued (Легист /
  Легист Про / Легист Энтерпрайз was the rough sketch from the discussion).
