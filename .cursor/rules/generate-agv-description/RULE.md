---
description: "AgnosticV catalog description generator - creates description.adoc for AgV catalog entries when user says 'generate agv description'"
alwaysApply: false
---

# AgnosticV Description Generator Skill

## Trigger Commands

When user says ANY of these phrases, invoke this skill:
- "generate agv description"
- "create agv description"
- "generate catalog description"
- "create catalog description"
- "generate description.adoc"
- "agv description"

## Skill Execution

**Action**: Read and follow `.claude/skills/generate-agv-description/SKILL.md` completely.

The skill file contains the complete workflow for:
1. Asking if user has a Showroom guide (FIRST - always ask)
2. Getting Showroom path if user chooses YES
3. Extracting module structure, technologies, and metadata
4. Gathering brief overview, warnings, guide link
5. Collecting featured technology list
6. Generating detailed agenda (optional)
7. Getting author and contact information
8. Determining output location
9. Generating and previewing description.adoc
10. Optional: Committing to git

## Important Notes

- ALWAYS ask "Do you have a Showroom?" first (option 1 or 2)
- Do NOT auto-detect and immediately read files without asking
- If user chooses YES: ask for Showroom path (can press Enter for current dir)
- If user chooses NO: proceed with manual entry
- Follow sequential questioning - ask ONE question at a time
- Use token management - write files with Write tool, show brief summaries only
- Preview description.adoc before writing
- Apply all critical rules from `showroom-standards` rule
