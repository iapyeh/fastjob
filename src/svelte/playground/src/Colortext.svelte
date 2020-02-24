<script>
	import marked from 'marked';
	export let name;
	export let colors = ['blue','red'];
	var count = 0;
	var change = () =>{
		count += 1
		setTimeout(change,1000)
	}
	setTimeout(change,1000)
	let value = "this is a book"
</script>

<style>
	h1 {
		color: purple;
	}
</style>

<h1 style="color:{(count%2) === 1 ? colors[0] : colors[1]}">Hello hi {name} {count%2}!</h1>
<textarea bind:value={value} style="width:300px;height:50px"></textarea>
<!--
-->
{@html marked(value)}
