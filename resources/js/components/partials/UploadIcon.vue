<template>
    <div>
      <div v-for="image in images" :key="image.alt" class="image-container d-inline-block">
        <img class="img-thumbnail zoomable-image" :alt="image.alt" :src="image.src">
      </div>
    </div>
  </template>
  
  <script>
  export default {
    data() {
      return {
        images: [
          { alt: 'giphy logo', src: '/storage/giphy-logo-vector.svg' },
          { alt: 'imgur logo', src: '/storage/imgur.svg' },
          { alt: 'gfycat-icon', src: '/storage/gfycat-icon.webp' },
          { alt: 'mp4-icon', src: '/storage/mp4-icon.png' },
          { alt: 'video-icon', src: '/storage/video-icon.png' },
          { alt: 'gif-icon', src: '/storage/gif-icon.png' },
          { alt: 'jpg-icon', src: '/storage/jpg-icon.png' },
          { alt: 'image-icon', src: '/storage/image-icon.png' },
        ],
      };
    },
  };
  </script>