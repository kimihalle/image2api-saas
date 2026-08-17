<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
const router = useRouter(); const auth = useAuthStore(); onMounted(() => { auth.openLogin(''); router.replace('/') })
</script>
<template><div></div></template>
