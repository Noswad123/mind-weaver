<script lang="ts">
	import { onMount } from 'svelte';
	let hoveredNote: { title: string } | null = null;
	let notes: { path: string; title: string; polarity: number }[] = [];

	onMount(async () => {
		const res = await fetch('/api/notes');
		notes = await res.json();
	});
	function generateRadialPolarityGrid(notes: { polarity: number }[], maxRadius: number) {
		const goldenAngle = Math.PI * (3 - Math.sqrt(5));
		const points = [];

		const maxPolarity = Math.max(...notes.map((n) => n.polarity || 0));

		for (let i = 0; i < notes.length; i++) {
			const polarity = notes[i].polarity || 0;
			const strength = 1 - polarity / maxPolarity; // 0 = strong, 1 = weak
			const radius = maxRadius * (0.2 + 0.8 * strength); // clamp to avoid full center

			const r = radius * Math.sqrt(i / notes.length);
			const theta = i * goldenAngle;

			const x = r * Math.cos(theta);
			const y = r * Math.sin(theta);

			points.push({ x, y });
		}

		return points;
	}
</script>

<h1>🧠</h1>
{#if hoveredNote}
	<h2>{hoveredNote.title}</h2>
{/if}

<svg viewBox="-250 -250 500 500" width="100%" height="1000">
	{#each generateRadialPolarityGrid(notes, 200) as pos, i}
		<circle
			class="note-dot"
			cx={pos.x}
			cy={pos.y}
			r={3 + (notes[i].polarity || 0) * 0.5}
			fill="deepskyblue"
			tabindex="0"
			role="button"
			on:mouseenter={() => (hoveredNote = notes[i])}
			on:focus={() => (hoveredNote = notes[i])}
			on:keydown={(e) => e.key === 'Enter' && (hoveredNote = notes[i])}
		>
			<title>{notes[i].title} ({notes[i].polarity})</title>
		</circle>
	{/each}
</svg>

<style>
	.note-dot {
		transition: transform 0.2s ease;
	}

	.note-dot:hover {
		transform: scale(1.3);
		cursor: pointer;
	}

	h2 {
		text-align: center;
		font-weight: 500;
		color: slateblue;
		margin-bottom: 1rem;
	}
</style>
