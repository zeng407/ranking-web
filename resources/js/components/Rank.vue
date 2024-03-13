<script>
export default {
  name: 'rank',
  methods: {
    clickTab: function (tab) {
      const urlParams = new URLSearchParams(window.location.search);
      urlParams.set('tab', tab);
      const newUrl = window.location.pathname + '?' + urlParams.toString();
      window.history.replaceState(null, null, newUrl);
    }
  }
}
</script>
