# Integraciones de producción

Los tokens viven únicamente en `.env.production`. Después de modificarlos se debe
recrear el backend con Docker Compose; no es necesario reconstruir el firmware.

## Uptime Kuma

Crear una notificación de tipo **Webhook** con estos valores:

- URL: `https://monitor.danielalbertorosso.com.ar/api/v1/events/uptime`
- método: `POST`
- body: preset JSON/default de Uptime Kuma (`monitor`, `heartbeat`, `msg`)
- header adicional: `Authorization: Bearer <UPTIME_KUMA_TOKEN>`

Asignar esa notificación a los monitores. Sus nombres deben coincidir con
`integrations.uptime_kuma.monitors` en
`contracts/config/platform.production.yaml`. La URL que Kuma ya conoce se guarda
automáticamente como endpoint del servicio; no debe duplicarse en YAML.

Un heartbeat `status=0` marca el servicio como caído, abre una alerta crítica y
publica MQTT. `status=1` lo marca operacional y resuelve la alerta abierta.

## GitHub Actions

En cada repositorio crear el secret `DESK_MONITOR_ACTIONS_TOKEN` con el mismo valor
que `GITHUB_ACTIONS_TOKEN` del servidor. Agregar al final del workflow:

```yaml
  notify-desk-monitor:
    if: always()
    needs: [deploy]
    runs-on: ubuntu-latest
    steps:
      - name: Notify Desk Monitor
        env:
          DESK_TOKEN: ${{ secrets.DESK_MONITOR_ACTIONS_TOKEN }}
          DEPLOY_RESULT: ${{ needs.deploy.result }}
        run: |
          curl --fail-with-body --retry 3 \
            -X POST https://monitor.danielalbertorosso.com.ar/api/v1/events/github/actions \
            -H "Authorization: Bearer $DESK_TOKEN" \
            -H "Idempotency-Key: $GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT-$DEPLOY_RESULT" \
            -H "Content-Type: application/json" \
            --data "$(jq -n \
              --arg product_key 'freshcode' \
              --arg repository "$GITHUB_REPOSITORY" \
              --arg branch "$GITHUB_REF_NAME" \
              --arg workflow "$GITHUB_WORKFLOW" \
              --arg status "$DEPLOY_RESULT" \
              --arg commit_sha "$GITHUB_SHA" \
              --arg author "$GITHUB_ACTOR" \
              --arg occurred_at "$(date -u +%FT%TZ)" \
              --arg html_url "$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID" \
              '{product_key:$product_key,repository:$repository,branch:$branch,workflow:$workflow,status:$status,commit_sha:$commit_sha,commit_message:"",author:$author,occurred_at:$occurred_at,html_url:$html_url}')"
```

Cambiar `needs: [deploy]` por el nombre real del job y `product_key` por la clave
del producto. Los resultados válidos son `success`, `failure`, `cancelled`,
`queued` e `in_progress`.

Como alternativa, GitHub puede enviar el evento `workflow_run` al endpoint
`/api/v1/events/github`; el secret configurado en GitHub debe coincidir con
`GITHUB_WEBHOOK_SECRET`.

## Verificación

Luego de disparar un evento, revisar el panel y los logs:

```bash
docker compose --env-file .env.production \
  -f docker-compose.yml -f docker-compose.prod.yml logs --tail=100 backend
```

Las respuestas aceptadas son HTTP `202`. Los reintentos con el mismo
`Idempotency-Key` no duplican datos.
