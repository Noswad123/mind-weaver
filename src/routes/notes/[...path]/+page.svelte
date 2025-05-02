<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';

  let note;

  $: path = $page.params.path;

  onMount(async () => {
    const res = await fetch(`/notes/${encodeURIComponent(path)}`);
    note = await res.json();
  });
</script>

{#if note}
  <h1>{note.title}</h1>
  <pre>{note.content}</pre>
{:else}
  <p>Loading...</p>
{/if}