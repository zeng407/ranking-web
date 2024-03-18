<template>
  <youtube 
    :ref="refId" 
    :video-id="videoId" 
    :width="width" 
    :height="height" 
    :autoplay="autoplay" 
    :controls="controls"
    :start="start" 
    :end="end" 
    :rel="rel" 
    :origin="origin"></youtube>
</template>

<script>
export default {
  props: {
    refId: {
      type: String,
      required: true
    },
    width: {
      type: String,
      default: '100%'
    },
    height: {
      type: String,
      default: '270'
    },
    videoId: {
      type: String,
      required: true
    },
    autoplay: {
      type: Boolean,
      default: false
    },
    controls: {
      type: Boolean,
      default: true
    },
    start: {
      default: Number || null
    },
    end: {
      default: Number || null
    },
    rel: {
      type: Number,
      default: 0
    },
    origin: {
      type: String,
      default: window.location.origin
    }
  }
};
</script>