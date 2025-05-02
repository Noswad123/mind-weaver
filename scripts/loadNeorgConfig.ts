import  fs from 'fs';
import  luaparse from 'luaparse';

function findNodeValue(node: any) {
  if (!node) return undefined;

  if (node.type === 'StringLiteral') {
    return node.raw.replace(/^['"]|['"]$/g, '');
  }

  if (node.type === 'TableConstructorExpression') {
    const obj: Record<string, any> = {};
    node.fields.forEach((field: any) => {
      if (field.type === 'TableKeyString') {
        obj[field.key.name] = findNodeValue(field.value);
      }
    });
    return obj;
  }

  return undefined;
}

function extractWorkspaces(ast: any) {
  let workspaces = {};

  function walk(node: any) {
    if (!node) return;

    if (node.type === 'ReturnStatement') {
      for (const arg of node.arguments) {
        if (arg.type === 'TableConstructorExpression') {
          for (const field of arg.fields) {
            if (
              field.key?.name === 'opts' &&
              field.value.type === 'TableConstructorExpression'
            ) {
              for (const loadField of field.value.fields) {
                if (
                  loadField.key?.name === 'load' &&
                  loadField.value.type === 'TableConstructorExpression'
                ) {
                  for (const pluginField of loadField.value.fields) {
                    if (
                      pluginField.key?.name === 'core.dirman' &&
                      pluginField.value.type === 'TableConstructorExpression'
                    ) {
                      const configField = pluginField.value.fields.find(
                        (f: any) => f.key?.name === 'config'
                      );

                      if (configField?.value?.type === 'TableConstructorExpression') {
                        for (const cf of configField.value.fields) {
                          if (cf.key?.name === 'workspaces') {
                            workspaces = findNodeValue(cf.value);
                          }
                        }
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }

  if (ast?.body?.length) {
    ast.body.forEach(walk);
  }

  return { workspaces };
}

function loadNeorgConfig(configPath: string) {
  const luaCode = fs.readFileSync(configPath, 'utf8');
  const ast = luaparse.parse(luaCode, { comments: false });
  return extractWorkspaces(ast);
}

export  { loadNeorgConfig};
