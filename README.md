# ThinK8sHook

Este repositorio implementa un Admission Webhook que adapta las cargas de trabajo a las necesidades del entorno ThinK8S.

## Mantenimiento

El código está basado en el ejemplo oficial publicado por kubernetes, https://github.com/kubernetes/kubernetes/tree/master/test/images/agnhost/webhook, reorganizado de la siguiente forma:

- El código original se conserva en `/internal/webhook`.
- Se elimina la función `init` de `/internal/webhook/main.go`, para evitar que registre comandos `Cobra` no deseados.
- Se añade al código original un fichero `internal/webhook/espose.go` que exporta las interfaces que son necesarias para utilizar las construcciones usadas en el código original.
- El nuevo código se añade a `/cmd`

Si se quiere actualizar el código base del hook, se debe copiar la última versión publicada por kubernetes a `/internal/webhook`, revisar cambios, y adecuar tanto `internal/webhook/main` como `internal/webhook/expose` a los cambios.

## Certificado

Para probar la aplicación, es necesario proporcionarle un certificado. Se puede generar un certificado autofirmado de prueba con el comando:

```bash
```
