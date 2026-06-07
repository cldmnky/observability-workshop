---
name: redhat-style-guide-validation
description: Validate AsciiDoc content against Red Hat corporate style guide. Use when reviewing capitalization, product names, inclusive language, hyphenation, numbers, and time/currency formatting. Returns a JSON report with compliance score and prioritized fixes.
license: Apache-2.0
compatibility: opencode
metadata:
  domain: content-quality
  audience: technical-writers, reviewers
---

# Red Hat Corporate Style Guide Validation

Use this validation framework to ensure all generated content follows Red Hat's official corporate style standards.

## Mandatory Red Hat Style Compliance

### 1. Capitalization (Red Hat's Distinctive Approach)

- **Headlines/Titles**: Sentence case only (not title case).
  - Correct: "Accelerating application development with Red Hat OpenShift".
  - Incorrect: "Accelerating Application Development With Red Hat OpenShift".
- **Product Names**: Follow official Red Hat product names list exactly.
- **Job Titles**: Capitalize when used as titles.
- **Department Names**: Use "team" not "business unit".

### 2. Red Hat Product Naming

- **Official Names Only**: reference official Red Hat product names list.
- **No "The"**: don't use "the Red Hat OpenShift Platform".
- **No Abbreviations**: avoid product acronyms in formal communications.
- **Proper Breaks**: keep "Red Hat" together on same line.

### 3. Writing Style and Tone

- **Numbers**: use numerals for ALL numbers (including under 10).
- **Contractions**: acceptable in marketing content, avoid in formal docs.
- **Serial Commas**: always use Oxford commas.
- **Hyphenation**:
  - Don't hyphenate: "open source", "hybrid cloud", "public cloud".
  - Do hyphenate: compound adjectives before nouns.

### 4. Language Precision

- Avoid vague terms: "robust", "powerful", "strong", "leverage".
- Avoid jargon: "actionable", "synergy", "game-changer".
- Avoid absolutes: "best", "leading", "most" (without citations).
- Use specific language: describe actual benefits and improvements.

### 5. Inclusive Language

- Prohibited: "whitelist/blacklist", "master/slave".
- Preferred: "allowlist/denylist", "primary/secondary".
- Gender-neutral: use "they/them" pronouns.
- Cultural sensitivity: avoid idioms that don't translate.

### 6. Technical Terminology

- Acronym expansion: spell out on first use.
- Command formatting: lowercase commands, proper case products.
- File extensions: all caps without periods (PDF not .pdf).
- Version numbers: major versions only in product names.

### 7. Time and Currency

- Time format: "4 PM" not "4:00 PM" or "4 p.m.".
- Time zones: specify clearly (ET, UTC).
- Currency: "US$1,500" for U.S. dollars.

## Blog-Specific Style Validation

### Marketing Blog Requirements

- Sentence-case headline with Red Hat product name.
- Business outcome focused.
- No superlatives without citations.
- Problem → Solution → Outcome flow.
- Specific metrics over vague benefits.
- Customer examples from approved sources.
- Red Hat product names used correctly.

### Developer Blog Requirements

- Commands in proper format.
- Code examples tested.
- Version-specific guidance.
- Accurate technical terminology.
- Red Hat product naming.
- Consistent hyphenation.
- Proper capitalization.
- Serial comma usage.

## Style Guide Validation Checklist (Pre-Publication Review)

- Headlines follow sentence case.
- Red Hat product names are official and correct.
- No prohibited language (vague, jargon, non-inclusive).
- Numbers formatted as numerals.
- Proper hyphenation applied.
- Citations follow Red Hat standards.
- Technical terms spelled correctly.
- Time/currency in Red Hat format.
- Contractions appropriate for content type.
- Voice and tone align with Red Hat personality.

## Common Red Hat Style Violations

- "The Red Hat OpenShift Platform" → "Red Hat OpenShift".
- "Five ways to improve" → "5 ways to improve".
- "Whitelist configuration" → "Allowlist configuration".
- "More secure solution" → "Security-focused solution with [specific features]".
- "Best-in-class platform" → "Leading platform by [specific metric + citation]".
- "Leverage our technology" → "Use our technology".

## Validation Scoring Framework

### Style Compliance Score (1–10)

- 10: Perfect Red Hat style compliance.
- 8–9: Minor style issues, easily corrected.
- 6–7: Several style violations, needs revision.
- 4–5: Significant style problems, major revision needed.
- 1–3: Does not follow Red Hat style standards.

### Critical Style Violations (Must Fix)

1. Incorrect product names.
2. Non-inclusive language.
3. Unsupported superlative claims.
4. Wrong capitalization in headlines.
5. Prohibited jargon or vague language.

### Recommended Style Improvements

1. Number formatting corrections.
2. Hyphenation adjustments.
3. Comma usage fixes.
4. Technical term clarifications.
5. Voice and tone refinements.

## Output Format

When applying this skill, emit a JSON report:

```json
{
  "compliance_score": 1-10,
  "critical_violations": [ {"category": "...", "location": "...", "before": "...", "after": "..."} ],
  "recommended_improvements": [ {"category": "...", "message": "..."} ],
  "files": ["path/to/file.adoc"]
}
```

Reference: based on the Red Hat Corporate Style Guide (confidential document for Red Hat associates and approved agencies).
