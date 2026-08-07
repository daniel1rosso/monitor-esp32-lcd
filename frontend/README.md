# Desk Monitor Frontend

Centro administrativo independiente construido con React, Vite y TypeScript. El
scaffold ya integra Tailwind, React Query, Framer Motion e i18n `es-AR`. El router
se incorporará cuando exista navegación y haya una release sin advisories altos.
Los componentes shadcn se incorporarán por composición durante la etapa
Frontend, sin copiar un kit completo antes de necesitarlo.

```bash
npm ci
npm run generate:api
npm run lint
npm test
npm run build
```

`src/api/schema.d.ts` se genera exclusivamente desde el OpenAPI raíz.
