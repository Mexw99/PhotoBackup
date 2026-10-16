<template>
  <header class="header">
    <NuxtLink class="brand" to="/"> PhotoBackup </NuxtLink>

    <nav class="menu">
      <NuxtLink to="/">Dashboard</NuxtLink>
      <NuxtLink to="/backup">Backup</NuxtLink>
      <NuxtLink to="/tags">Tag</NuxtLink>
      <NuxtLink to="/search">Search</NuxtLink>
    </nav>
  </header>
</template>

<style scoped>
.header {
  align-items: center;
  background: #ffffff;
  border-bottom: 1px solid #e5e7eb;
  display: flex;
  justify-content: space-between;
  padding: 18px 32px;
}

.brand {
  color: #2563eb;
  font-size: 1.25rem;
  font-weight: bold;
  text-decoration: none;
}

.menu {
  display: flex;
  gap: 10px;
}

.menu a {
  border-radius: 8px;
  color: #475569;
  padding: 8px 12px;
  text-decoration: none;
}

.menu a:hover,
.menu a.router-link-active {
  background: #eff6ff;
  color: #1d4ed8;
}

@media (max-width: 700px) {
  .header {
    align-items: flex-start;
    flex-direction: column;
    gap: 16px;
  }

  .menu {
    flex-wrap: wrap;
  }
}
</style>
