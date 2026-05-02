# Product Requirements Document (PRD): Critical - AD&D 2.5 Digital Framework

## 1. Project Overview
This web application, named **Critical**, is a modular, high-performance tool designed to digitize and automate the complex mechanics of AD&D 2.5 Edition. It solves the "math overhead" of tabletop gaming by automating state management for spells, combat arithmetic, and layered buffs. The initial implementation focuses on a Bard character to test both magic and combat systems, while maintaining a class-agnostic architecture for future expansions. 

## 2. Core Functional Requirements

### 2.1 Dynamic Character State Engine & Static Profile
The app must track fundamental attributes alongside real-time fluctuating vitals:
*   **Attribute Management:** Store the six core abilities: Strength, Dexterity, Constitution, Intelligence, Wisdom, and Charisma.
*   **Saving Throws & Skills:** Manage target numbers for saving throws (Paralyze/Poison, Rod/Staff/Wand, Petrify/Polymorph, Breath Weapon, Spells) and track percentage-based skills like Climb Wall, Hear Noise, Pickpocket, and Ancient Language.
*   **Vitals Tracking:** Provide real-time management of Hit Points (HP) with a ledger for damage and healing history that calculates current life against the maximum.
*   **Defense Logic:** Calculate Armor Class (AC) modifiers factoring in base stats, touch AC, and active protections like Protection from Evil.
*   **Combat Stats:** Track base THAC0 and dynamically apply modifiers.

### 2.2 Modular Spell & Power Management
A system to manage spellbooks, daily memorization, and casting limits.
*   **Daily Preparation & Usage Tracking:** Allow selection of "Known" spells into "Prepared" slots based on class level limits. Include a one-tap "Cast" functionality that decrements available slots and shows the remaining slots.
*   **Metadata Integration:** Connect to a spell database to display casting times, ranges, and area-of-effect details.
*   **Scrolls & Spell Learning:** Track magical scrolls, their spell levels, pages required, and the historical percentage chance of successfully learning the spell (e.g., Cone of Cold).

### 2.3 Semantic Dice & Rule Engine
An integrated dice roller that understands context and automates rule enforcement.
*   **Context-Aware Rolling:** Automate success logic by using "Roll Under" for attributes/skills and "Roll Over" for saves and THAC0 attacks. Explicitly report the margin of success or failure.
*   **Arithmetic Parser:** Support flexible linear formulas utilizing offsets, multipliers, and multi-die types.
*   **Combat Integration:** Process automated weapon damage profiles tailored to target sizes, distinguishing Small/Medium versus Large damage for weapons like Bastard Swords, Short Bows, and Darts.

### 2.4 Buff, Effect Layering & Inventory
*   **Layered Defenses & Temporal Effects:** Manage multi-hit protections by tracking remaining charges (e.g., images for Stoneskin) and track duration-based expirations.
*   **Modifier Persistence:** Apply global modifiers when an effect is active, such as protection spells granting saving throw bonuses and damage reduction from specific elements.
*   **Conditional Item Logic:** Support complex artifact rules like the Power Gem, which provides telepathy, casts Dispel Magic, and transforms weapons (e.g., into a thunder hammer that stuns in an area of effect for amplified damage).
*   **Inventory Management:** Track equipped gear (e.g., Bastard Sword, Short Bow) versus carried valuables. Manage finite resources like specific magical arrows, flasks of burning fire, diamonds, and charged items like magic wands.

### 2.5 Experience (XP) & Progression Tracker
*   **XP Ledger & Leveling:** Maintain a historical ledger of Experience Points gained per session by date. Track total XP against class-specific thresholds and automatically display the percentage progression towards the next level requirement.

## 3. Technical Stack
*   **Backend (Go):** A centralized Go logic engine handling all mechanical calculations with strongly typed structs (Characters, Spells, Items) to prevent data corruption, accessible via RESTful or GraphQL endpoints.
*   **Frontend (React/Vue/Svelte):** A responsive UI optimized for mobile and tablet use at the gaming table.
*   **Database:** PostgreSQL for the global spell/item database, utilizing local caching for offline session resilience.

## 4. Future Roadmap
*   **Priest & Psionics Modules:** Future support for faith-based Spheres (Phase 2) and Psionic Strength Point (PSP) pools and Mental THAC0 systems (Phase 3).
*   **Multi-Character Support:** Allow a single user to manage multiple sheets, or enable a DM to view player states.
*   **NPC/Ally Database:** Track known NPCs, enemies, and allies, such as the Ogre Mage chief, the Pit Fiend, and the Elf King.

