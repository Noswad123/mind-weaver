<script lang="ts">
  let notes = [];
  import { onMount } from 'svelte';

  onMount(async () => {
    const res = await fetch('/api/notes');
    notes = await res.json();
  });
</script>

<h1>All Notes</h1>
<ul>
  {#each notes as note}
    <li><a href={`/notes/${encodeURIComponent(note.path)}`}>{note.title}</a></li>
  {/each}
</ul>