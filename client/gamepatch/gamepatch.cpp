// GamePatch.dll — o visual do tooltip de montaria do WYD.exe 7662: a paleta das
// linhas, o fundo e a borda.
//
// O executável já carrega este DLL sozinho: o ponto de entrada foi desviado para
// um carregador (0x5F3C66) que chama LoadLibraryA("GamePatch.dll"), depois
// "Shield.dll", depois "ClientPatch.dll", e só então entra no jogo. Nenhum dos
// dois primeiros existe no cliente, então basta este arquivo estar na pasta —
// sem mexer no WYD.exe nem no ClientPatch.dll (cujo código no repositório não é
// exatamente o que está compilado no cliente, e por isso não é recompilado).
//
// O que ele faz: envolve a função que monta o tooltip de item (0x416A80). O
// cliente monta tudo normalmente; no fim, se o item for uma montaria adulta
// (2360-2389, índice lido do slot sob o mouse), as linhas ganham a paleta da
// montaria, o fundo troca de cor e uma borda é desenhada em volta. Espada,
// armadura e todo o resto continuam como o cliente desenha.
//
// Como ele sabe o texto de cada linha sem decifrar onde o controle guarda a
// string: intercepta o método SetText (vtable +0x80) das 13 linhas do tooltip e
// anota o texto no momento em que o cliente o escreve. A cor é aplicada depois,
// pelo próprio SetColor (vtable +0x84) do controle.
//
// Tudo o que ele grava na memória é conferido antes: se os bytes do cliente não
// forem os da build 7662, ele não faz nada. Um tooltip sem cor é um incômodo;
// um gancho no lugar errado derruba o jogo.

#include <windows.h>
#include <math.h>
#include <string.h>

namespace {

// Endereços da build 7662 (WYD.exe de 2 347 008 bytes).
constexpr DWORD kTooltipFunc = 0x416A80; // função que monta o tooltip de item
constexpr DWORD kUIRoot = 0x6F0AB0;      // ponteiro para a janela do tooltip
constexpr DWORD kTitleOffset = 0x27A08;  // controle do título (o nome do item)
constexpr DWORD kLinesOffset = 0x27A10;  // 14 linhas: 13 de corpo e o preço por último
constexpr int kBodyLines = 14;           // controles 0x305..0x321
constexpr int kTracked = 1 + kBodyLines; // índice 0 é o título
constexpr int kSetTextSlot = 0x80 / 4;
constexpr int kSetColorSlot = 0x84 / 4;

// Os oito primeiros bytes da função: push ebp / mov ebp,esp / mov eax,0x1498.
constexpr BYTE kExpectedPrologue[8] = {0x55, 0x8B, 0xEC, 0xB8, 0x98, 0x14, 0x00, 0x00};

// Onde a função pega o slot sob o mouse: logo depois da chamada virtual +0xB8
// (x, y), "mov [ebp-8], eax / mov eax, [0x6F0AB0]". Um desvio ali anota o índice
// do item do slot — [slot+0x670] é o STRUCT_ITEM, e o índice é o primeiro word,
// lido pelo próprio cliente em 0x416BA6. É assim que o tooltip sabe que é de
// montaria, com ou sem linha de absorção.
constexpr DWORD kSlotHookAt = 0x416B4E;
constexpr DWORD kSlotHookBack = 0x416B56;
constexpr BYTE kExpectedSlotBytes[8] = {0x89, 0x45, 0xF8, 0xA1, 0xB0, 0x0A, 0x6F, 0x00};
constexpr int kAdultMountLo = 2360; // Porco
constexpr int kAdultMountHi = 2389; // Pantera Negra

// A paleta do tooltip de montaria, em ARGB. O título não tem rótulo: é o nome do
// item, e fica com a sua cor sempre que o tooltip for de montaria.
constexpr DWORD kColorTitle = 0xFFE0F7FF;

// Cada linha é reconhecida pelo começo do texto, como o cliente o escreve
// (Windows-1252). A ordem importa onde um rótulo é prefixo de outro: "Level
// necessário" vem antes de "Level :".
struct LineColor {
    const char* prefix;
    DWORD color;
};
const LineColor kLineColors[] = {
    {"Level necess", 0xFF6E7C8C},
    {"Aumento de Dano", 0xFFFF9E64},
    {"Aumento de Imunidades", 0xFFFDE68A},
    {"\xcdndice de Evas\xe3o", 0xFFFDE68A},
    {"Ataque M\xe1gico", 0xFFA78BFA},
    {"Absor\xe7\xe3o", 0xFF5EEAD4},
    {"Vitalidade", 0xFFCBD5E1},
    {"HP :", 0xFFCBD5E1},
    {"Level :", 0xFFCBD5E1},
    {"Ra\xe7\xe3o", 0xFFCBD5E1},
    {"Pre\xe7o", 0xFF7DD3FC},
};

// Só o tooltip de montaria tem estas linhas; é por elas que ele é reconhecido.
const char kAbsorbPrefix[] = "Absor\xe7\xe3o";

typedef void(__fastcall* SetTextFn)(void* self, void* edx, const char* text, int flag);
typedef void(__fastcall* SetColorFn)(void* self, void* edx, DWORD color);

// Cada classe de controle tem a sua vtable; os controles do tooltip podem não
// ser todos da mesma. Guarda os métodos originais de cada vtable trocada.
struct HookedVtable {
    void** vtable;
    SetTextFn setText;
    SetColorFn setColor;
};
HookedVtable g_vtables[8];
int g_vtableCount = 0;
bool g_controlsHooked = false;

// Estado do tooltip que está sendo montado.
DWORD g_wantedColor[kTracked]; // a cor da paleta para a linha, ou 0
bool g_lineUsed[kTracked];     // a linha recebeu texto não vazio neste tooltip
char g_title[64];              // o nome do item, como o cliente o escreveu no título
DWORD g_clientColor[kTracked]; // a cor que o próprio cliente escolheu
bool g_tooltipHasAbsorb = false;
// O índice do item do tooltip, anotado pelo desvio em kSlotHookAt; -1 sem item.
// Sem o desvio (bytes de outra build), a montaria é reconhecida pela linha de
// absorção, como antes.
int g_tooltipItem = -1;
bool g_slotHooked = false;
// O cliente chama a função do tooltip o tempo todo — umas vinte vezes por quadro
// desenhado —, e quase sempre ela sai logo no começo sem montar nada. Só a
// chamada que escreve texto nas linhas monta um tooltip; as outras não podem
// mexer no estado, ou apagam o fundo e a borda da montaria antes do desenho.
bool g_tooltipBuilt = false;
// true entre o começo e o fim da função do tooltip de item. Texto escrito nas
// linhas fora dela é outro tooltip — o de skill usa as mesmas linhas e o mesmo
// painel — e aí o estado de montaria tem de ser desfeito.
bool g_insideTooltip = false;
void ForgetMountTooltip();
bool g_applying = false; // true enquanto este DLL pinta: não conta como escolha do cliente

bool StartsWith(const char* text, const char* prefix) {
    return text != nullptr && strncmp(text, prefix, strlen(prefix)) == 0;
}

// O controle rastreado de índice i: 0 é o título, 1..14 as linhas do corpo.
void* TrackedControl(int i) {
    DWORD root = *reinterpret_cast<DWORD*>(kUIRoot);
    if (root == 0) {
        return nullptr;
    }
    DWORD at = i == 0 ? root + kTitleOffset : root + kLinesOffset + 4 * (i - 1);
    return *reinterpret_cast<void**>(at);
}

int TrackedIndexOf(void* control) {
    for (int i = 0; i < kTracked; i++) {
        if (TrackedControl(i) == control) {
            return i;
        }
    }
    return -1;
}

HookedVtable* HookFor(void* control) {
    void** vtable = *reinterpret_cast<void***>(control);
    for (int i = 0; i < g_vtableCount; i++) {
        if (g_vtables[i].vtable == vtable) {
            return &g_vtables[i];
        }
    }
    return nullptr;
}

// Um aviso do próprio cliente — o "Level necessário" em vermelho quando falta
// nível, por exemplo — é informação de jogo e fica como está. Vermelho aqui é
// qualquer tom com o vermelho alto e o verde e o azul baixos.
bool IsWarningColor(DWORD argb) {
    DWORD r = (argb >> 16) & 0xFF, g = (argb >> 8) & 0xFF, b = argb & 0xFF;
    return r >= 0xC0 && g < 0x60 && b < 0x60;
}

void __fastcall HookedSetText(void* self, void* edx, const char* text, int flag) {
    int i = TrackedIndexOf(self);
    if (i >= 0) {
        if (g_insideTooltip) {
            g_tooltipBuilt = true;
        } else {
            ForgetMountTooltip();
        }
        g_lineUsed[i] = text != nullptr && text[0] != 0;
        if (i == 0 && g_insideTooltip) {
            strncpy_s(g_title, sizeof(g_title), text != nullptr ? text : "", _TRUNCATE);
        }
        DWORD color = 0;
        if (i == 0) {
            color = kColorTitle;
        } else {
            for (const LineColor& lc : kLineColors) {
                if (StartsWith(text, lc.prefix)) {
                    color = lc.color;
                    break;
                }
            }
        }
        if (StartsWith(text, kAbsorbPrefix)) {
            g_tooltipHasAbsorb = true;
        }
        g_wantedColor[i] = color;
    }
    HookedVtable* hook = HookFor(self);
    if (hook != nullptr) {
        hook->setText(self, edx, text, flag);
    }
}

void __fastcall HookedSetColor(void* self, void* edx, DWORD color) {
    if (!g_applying) {
        int i = TrackedIndexOf(self);
        if (i >= 0) {
            g_clientColor[i] = color;
        }
    }
    HookedVtable* hook = HookFor(self);
    if (hook != nullptr) {
        hook->setColor(self, edx, color);
    }
}

// Troca SetText e SetColor nas vtables dos controles do tooltip. Feito na
// primeira vez que um tooltip é aberto, porque é só então que eles existem.
void HookTooltipControls() {
    for (int i = 0; i < kTracked; i++) {
        if (TrackedControl(i) == nullptr) {
            return; // a janela ainda não terminou de ser montada; tenta no próximo
        }
    }
    for (int i = 0; i < kTracked; i++) {
        void** vtable = *reinterpret_cast<void***>(TrackedControl(i));
        if (HookFor(TrackedControl(i)) != nullptr) {
            continue;
        }
        if (g_vtableCount == ARRAYSIZE(g_vtables)) {
            return;
        }
        DWORD oldProtect = 0;
        if (!VirtualProtect(&vtable[kSetTextSlot], 2 * sizeof(void*), PAGE_READWRITE, &oldProtect)) {
            return;
        }
        HookedVtable& h = g_vtables[g_vtableCount++];
        h.vtable = vtable;
        h.setText = reinterpret_cast<SetTextFn>(vtable[kSetTextSlot]);
        h.setColor = reinterpret_cast<SetColorFn>(vtable[kSetColorSlot]);
        vtable[kSetTextSlot] = reinterpret_cast<void*>(&HookedSetText);
        vtable[kSetColorSlot] = reinterpret_cast<void*>(&HookedSetColor);
        VirtualProtect(&vtable[kSetTextSlot], 2 * sizeof(void*), oldProtect, &oldProtect);
    }
    g_controlsHooked = true;
}


// --- Fundo e borda do tooltip de montaria --------------------------------------
//
// O tooltip inteiro é um painel (controle 0x102, em +0x58 da janela) que se
// desenha como UM retângulo de cor, sem textura, com a cor em +0x94 (o cliente
// usa 0xAA000000, preto a 67%). O Render dele (vtable +0x58, 0x4015F7) monta um
// nó de desenho em +0x64 do painel — retângulo na tela em +0x68..+0x74, cor em
// +0x94 — e o encadeia na lista da camada pela função 0x40C43D, que guarda o
// PONTEIRO (o próximo nó fica em +0x150 do nó).
//
// Na montaria, esse retângulo reto não serve: os cantos do fundo apareceriam
// fora da borda arredondada. Então, depois que o Render entrega o nó do cliente,
// o tamanho dele é zerado — ele não aparece —, e o fundo e a borda são
// desenhados aqui, com cantos redondos, como nós fixos deste DLL (cópias do nó do
// cliente com retângulo e cor trocados). No quadro seguinte o Render recalcula o
// tamanho a partir do painel, e o ciclo se repete.
//
// A borda é um degradê da cor da raridade, com um reflexo claro que corre em
// volta do tooltip — o efeito de um .gif, calculado a cada quadro.

constexpr DWORD kPanelOffset = 0x58;      // controle 0x102: o painel do tooltip
constexpr DWORD kPanelColor = 0x94;       // cor do retângulo do painel
constexpr DWORD kPanelNode = 0x64;        // o nó de desenho dentro do painel
// O nó inteiro, até +0x168 inclusive (o construtor em 0x40BF80 inicia até lá).
// Não basta ir até o ponteiro do próximo nó: o desenho (0x43511A) lê +0x160, o
// índice de textura (-1 = nenhuma), e com a cópia curta ele lia o começo do nó
// seguinte do nosso vetor — no último nó, zeros, "textura 0", e o pedaço não
// era desenhado. Era a faixa de fundo de baixo que sumia.
constexpr DWORD kNodeSize = 0x16C;
constexpr DWORD kNodeRect = 0x04;         // x, y, largura, altura (float), na tela
constexpr DWORD kNodeColor = 0x30;        // = painel +0x94
constexpr DWORD kNodeNext = 0x150;        // próximo nó da lista
constexpr int kPanelRenderSlot = 0x58 / 4;
constexpr DWORD kAppendNode = 0x40C43D;   // int __cdecl (lista, nó, camada)
constexpr int kLayers = 30;               // a entrega recusa camada >= 30

// rgb(12,18,25), opaco: o fundo é pintado por cima da sobra da borda (ver
// DrawRoundedTooltip), e qualquer transparência deixaria o dourado vazar.
constexpr DWORD kColorBackground = 0xFF0C1219;

// A forma.
constexpr float kBorderWidth = 5.0f;       // px
constexpr int kCornerRadius = 14;          // px; o raio de dentro fica em 14-5
constexpr int kSegmentsPerSide = 32;       // pedaços por lado reto: mais pedaços, degradê mais liso
constexpr DWORD kShinePeriodMs = 2400;     // uma volta completa do reflexo
// Linhas dos quatro cantos de fora e de dentro, pedaços das retas e as três
// faixas do fundo.
constexpr int kMaxNodes = 4 * kCornerRadius * 2 + 4 * kSegmentsPerSide + 3;

// As famílias de cor da raridade: o tom escuro da borda e o tom claro do
// reflexo (e da linha do nível), do mesmo matiz. O azul é o da paleta.
struct Family {
    const char* name;
    BYTE dark[3], light[3];
};
const Family kFamilies[] = {
    {"cinza", {58, 62, 68}, {176, 182, 190}},
    {"verde", {28, 74, 44}, {110, 231, 150}},
    {"azul", {43, 68, 89}, {116, 185, 242}},
    {"roxo", {66, 32, 104}, {196, 148, 255}},
    {"dourado", {110, 76, 18}, {255, 214, 102}},
    {"laranja", {115, 52, 14}, {255, 158, 72}},
    {"vermelho", {105, 18, 26}, {255, 92, 92}},
};
constexpr int kDefaultFamily = 0; // montaria sem raridade no arquivo: cinza, sem linha de nível

typedef void(__fastcall* PanelRenderFn)(void* self, void* edx, void* list, float x, float y, int layer, int extra);
typedef int(__cdecl* AppendNodeFn)(void* list, void* node, int layer);

PanelRenderFn g_panelRender = nullptr; // o Render original da classe do painel
DWORD g_panelOriginalColor = 0;        // a cor que o cliente dá ao painel
bool g_mountTooltip = false;           // o último tooltip montado era de montaria
int g_family = kDefaultFamily;         // a família de cor desse tooltip
BYTE g_nodes[kMaxNodes][kNodeSize];

void* TooltipPanel() {
    DWORD root = *reinterpret_cast<DWORD*>(kUIRoot);
    return root == 0 ? nullptr : *reinterpret_cast<void**>(root + kPanelOffset);
}

DWORD& PanelColor(void* panel) {
    return *reinterpret_cast<DWORD*>(reinterpret_cast<BYTE*>(panel) + kPanelColor);
}

DWORD FamilyColor(int family, float k) {
    const Family& f = kFamilies[family];
    DWORD rgb = 0;
    for (int c = 0; c < 3; c++) {
        float v = f.dark[c] + (f.light[c] - f.dark[c]) * k;
        rgb = (rgb << 8) | (static_cast<DWORD>(v + 0.5f) & 0xFF);
    }
    return 0xFF000000 | rgb;
}

// A cor da borda num ponto do contorno (0..1, no sentido horário), com o reflexo
// na posição phase: um cosseno ao quadrado — claro no centro, descendo suave até
// o tom escuro, sem emenda na volta.
DWORD BorderShade(float along, float phase) {
    float k = 0.5f + 0.5f * cosf(6.2831853f * (along - phase));
    return FamilyColor(g_family, k * k);
}

// Monta os nós de uma volta: primeiro o fundo, depois a borda por cima.
struct NodeBuilder {
    const BYTE* tmpl;
    void* list;
    int layer;
    int count;

    void Add(float x, float y, float w, float h, DWORD color) {
        if (count == kMaxNodes || w <= 0 || h <= 0) {
            return;
        }
        BYTE* b = g_nodes[count++];
        memcpy(b, tmpl, kNodeSize);
        float r[4] = {x, y, w, h};
        memcpy(b + kNodeRect, r, sizeof(r));
        *reinterpret_cast<DWORD*>(b + kNodeColor) = color;
        *reinterpret_cast<DWORD*>(b + kNodeNext) = 0;
        reinterpret_cast<AppendNodeFn>(kAppendNode)(list, b, layer);
    }
};

// Meia largura de um arco de raio r na linha que fica a dy do centro, já em
// pixel inteiro.
float ArcSpan(float r, float dy) {
    return dy < r ? floorf(sqrtf(r * r - dy * dy) + 0.5f) : 0.0f;
}

// O tooltip é pintado em duas camadas, como tinta: primeiro um arredondado
// cheio com as cores da borda, depois o arredondado de dentro, com o fundo, por
// cima. A borda é o que sobra entre os dois.
//
// Cada peça passa um pixel da vizinha, sempre na direção do miolo. O cliente
// arredonda posição e tamanho de cada retângulo por conta própria, e peças que
// só encostam abrem fresta de 1 px conforme o arredondamento — foi o que
// apareceu nos cantos. Com a sobra, a fresta não tem onde abrir; e como a sobra
// vai sempre para dentro, o que passa do anel fica debaixo do fundo, que é
// opaco e é pintado depois.
void DrawRoundedTooltip(const BYTE* node, void* list, int layer) {
    const float* rect = reinterpret_cast<const float*>(node + kNodeRect);
    // Pixel inteiro, sempre para DENTRO do retângulo do cliente: ele fica em
    // coordenada quebrada (y = 315,39), e arredondar para o mais próximo punha a
    // última linha da borda até 1 px além do fim dele — e essa linha não aparecia.
    const float px = ceilf(rect[0] - 0.01f), py = ceilf(rect[1] - 0.01f);
    const float w = floorf(rect[0] + rect[2] + 0.01f) - px, h = floorf(rect[1] + rect[3] + 0.01f) - py;
    const float t = kBorderWidth;
    const int R = kCornerRadius, r = kCornerRadius - static_cast<int>(kBorderWidth);
    const float Rf = static_cast<float>(R), rf = static_cast<float>(r);
    if (w < 3 * Rf || h < 3 * Rf) {
        return;
    }
    NodeBuilder nb{node, list, layer, 0};

    // Comprimentos do contorno, no sentido horário a partir da ponta esquerda do
    // lado de cima: reta de cima, canto, reta da direita, canto, reta de baixo,
    // canto, reta da esquerda, canto.
    const float Lw = w - 2 * Rf, Lh = h - 2 * Rf, Lc = 1.5707963f * Rf;
    const float perimeter = 2 * Lw + 2 * Lh + 4 * Lc;
    const float phase = static_cast<float>(GetTickCount() % kShinePeriodMs) / kShinePeriodMs;
    const float arcStart[4] = {Lw, Lw + Lc + Lh, 2 * Lw + 2 * Lc + Lh, 2 * Lw + 3 * Lc + 2 * Lh}; // TR, BR, BL, TL

    // 1) O arredondado de fora, com as cores da borda. Cada canto é um quarto de
    // disco, linha a linha, do contorno até o centro do círculo (+1 px); cada
    // linha desce (cantos de cima) ou sobe (cantos de baixo) 1 px, que é o lado
    // em que o disco alarga — nunca sai da curva.
    for (int corner = 0; corner < 4; corner++) {
        const bool right = corner == 0 || corner == 1;
        const bool top = corner == 0 || corner == 3;
        const float cx = right ? px + w - Rf : px + Rf;
        const float boxY = top ? py : py + h - Rf;
        for (int j = 0; j < R; j++) {
            const float dy = top ? Rf - j - 0.5f : j + 0.5f; // do centro ao meio da linha
            const float xo = ArcSpan(Rf, dy);
            const float y = top ? boxY + j : boxY + j - 1;
            // Onde esta linha cai no arco, no sentido horário: nos cantos da
            // direita o arco desce (a primeira linha é o começo dele); nos da
            // esquerda, sobe (a primeira linha é o fim).
            const float frac = right ? (j + 0.5f) / R : 1.0f - (j + 0.5f) / R;
            const DWORD ring = BorderShade((arcStart[corner] + frac * Lc) / perimeter, phase);
            nb.Add(right ? cx - 1 : cx - xo, y, xo + 1, 2.0f, ring);
        }
    }
    // As quatro retas, em pedaços para o degradê. Cada pedaço vai até o começo
    // do seguinte +1 px, e 1 px mais grosso para dentro.
    for (int i = 0; i < kSegmentsPerSide; i++) {
        const float a0 = floorf(i * Lw / kSegmentsPerSide), a1 = floorf((i + 1) * Lw / kSegmentsPerSide);
        const float b0 = floorf(i * Lh / kSegmentsPerSide), b1 = floorf((i + 1) * Lh / kSegmentsPerSide);
        const float mw = (i + 0.5f) * Lw / kSegmentsPerSide, mh = (i + 0.5f) * Lh / kSegmentsPerSide;
        nb.Add(px + Rf + a0, py, a1 - a0 + 1, t + 1,
               BorderShade(mw / perimeter, phase)); // em cima, da esquerda para a direita
        nb.Add(px + w - t - 1, py + Rf + b0, t + 1, b1 - b0 + 1,
               BorderShade((Lw + Lc + mh) / perimeter, phase)); // direita, de cima para baixo
        nb.Add(px + w - Rf - a1, py + h - t - 1, a1 - a0 + 1, t + 1,
               BorderShade((Lw + 2 * Lc + Lh + mw) / perimeter, phase)); // embaixo, da direita para a esquerda
        nb.Add(px, py + h - Rf - b1, t + 1, b1 - b0 + 1,
               BorderShade((2 * Lw + 3 * Lc + Lh + mh) / perimeter, phase)); // esquerda, de baixo para cima
    }

    // 2) O arredondado de dentro, com o fundo: recuado a espessura da borda, com
    // o raio de dentro e o mesmo centro dos cantos. As faixas também se cobrem
    // em 1 px entre si — fundo sobre fundo, opaco, não aparece.
    for (int corner = 0; corner < 4; corner++) {
        const bool right = corner == 0 || corner == 1;
        const bool top = corner == 0 || corner == 3;
        const float cx = right ? px + w - Rf : px + Rf;
        const float boxY = top ? py + t : py + h - t - rf;
        for (int k = 0; k < r; k++) {
            const float dy = top ? rf - k - 0.5f : k + 0.5f;
            const float xi = ArcSpan(rf, dy);
            const float y = top ? boxY + k : boxY + k - 1;
            nb.Add(right ? cx - 1 : cx - xi, y, xi + 1, 2.0f, kColorBackground);
        }
    }
    nb.Add(px + Rf - 1, py + t, Lw + 2, rf + 1, kColorBackground);          // faixa de cima
    nb.Add(px + t, py + Rf, w - 2 * t, Lh, kColorBackground);               // faixa do meio
    nb.Add(px + Rf - 1, py + h - Rf - 1, Lw + 2, rf + 1, kColorBackground); // faixa de baixo
}

void __fastcall HookedPanelRender(void* self, void* edx, void* list, float x, float y, int layer, int extra) {
    const bool mine = g_mountTooltip && self == TooltipPanel() && layer >= 0 && layer < kLayers;
    g_panelRender(self, edx, list, x, y, layer, extra);
    if (!mine) {
        return;
    }
    // O Render só entrega o nó quando o painel tem cor e cabe na tela, e a
    // entrega o põe como último da lista da camada (lista + 8*camada: início,
    // fim). Sem fundo do cliente neste quadro, não há o que substituir.
    BYTE* node = reinterpret_cast<BYTE*>(self) + kPanelNode;
    void* tail = *reinterpret_cast<void**>(reinterpret_cast<BYTE*>(list) + 8 * layer + 4);
    if (tail != node) {
        return;
    }
    DrawRoundedTooltip(node, list, layer);
    // O retângulo reto do cliente some: largura e altura zero neste quadro.
    float* rect = reinterpret_cast<float*>(node + kNodeRect);
    rect[2] = 0.0f;
    rect[3] = 0.0f;
}

// Troca o Render da classe do painel. A vtable é compartilhada por outros
// painéis da interface; o gancho só age no painel do tooltip.
void HookTooltipPanel() {
    void* panel = TooltipPanel();
    if (panel == nullptr || g_panelRender != nullptr) {
        return;
    }
    g_panelOriginalColor = PanelColor(panel);
    void** vtable = *reinterpret_cast<void***>(panel);
    DWORD oldProtect = 0;
    if (!VirtualProtect(&vtable[kPanelRenderSlot], sizeof(void*), PAGE_READWRITE, &oldProtect)) {
        return;
    }
    g_panelRender = reinterpret_cast<PanelRenderFn>(vtable[kPanelRenderSlot]);
    vtable[kPanelRenderSlot] = reinterpret_cast<void*>(&HookedPanelRender);
    VirtualProtect(&vtable[kPanelRenderSlot], sizeof(void*), oldProtect, &oldProtect);
}

// --- Raridade das montarias ------------------------------------------------------
//
// Lida de GamePatch.txt, na pasta do jogo, uma montaria por linha:
//
//     Andaluz B = Divina, dourado
//
// O nome é o do título do tooltip; depois do "=" vêm o nome do nível, que
// aparece numa linha do tooltip ("Montaria nível Divina"), e a família de cor
// da borda (cinza, verde, azul, roxo, dourado, laranja, vermelho). O arquivo é texto
// no código de página do cliente (Windows-1252). Montaria que não está nele fica
// com a borda cinza e sem a linha de nível.

struct Rarity {
    char mount[48];
    char label[40];
    int family;
};
Rarity g_rarities[64];
int g_rarityCount = 0;
bool g_raritiesLoaded = false;

void Trim(char* s) {
    char* start = s;
    while (*start == ' ' || *start == '\t') {
        start++;
    }
    size_t n = strlen(start);
    while (n > 0 && (start[n - 1] == ' ' || start[n - 1] == '\t' || start[n - 1] == '\r' || start[n - 1] == '\n')) {
        n--;
    }
    memmove(s, start, n);
    s[n] = 0;
}

void LoadRarities() {
    g_raritiesLoaded = true;
    HANDLE f = CreateFileA("GamePatch.txt", GENERIC_READ, FILE_SHARE_READ, nullptr, OPEN_EXISTING,
                           FILE_ATTRIBUTE_NORMAL, nullptr);
    if (f == INVALID_HANDLE_VALUE) {
        return;
    }
    static char data[16384];
    DWORD read = 0;
    ReadFile(f, data, sizeof(data) - 1, &read, nullptr);
    CloseHandle(f);
    data[read] = 0;

    char* line = data;
    while (line != nullptr && *line != 0 && g_rarityCount < ARRAYSIZE(g_rarities)) {
        char* next = strchr(line, '\n');
        if (next != nullptr) {
            *next++ = 0;
        }
        char* eq = strchr(line, '=');
        if (line[0] != '#' && eq != nullptr) {
            *eq = 0;
            Rarity& r = g_rarities[g_rarityCount];
            strncpy_s(r.mount, sizeof(r.mount), line, _TRUNCATE);
            Trim(r.mount);
            char* value = eq + 1;
            char* comma = strchr(value, ',');
            char colorName[32] = "";
            if (comma != nullptr) {
                *comma = 0;
                strncpy_s(colorName, sizeof(colorName), comma + 1, _TRUNCATE);
            }
            strncpy_s(r.label, sizeof(r.label), value, _TRUNCATE);
            Trim(r.label);
            Trim(colorName);
            r.family = kDefaultFamily;
            for (int i = 0; i < ARRAYSIZE(kFamilies); i++) {
                if (_stricmp(colorName, kFamilies[i].name) == 0) {
                    r.family = i;
                }
            }
            if (r.mount[0] != 0) {
                g_rarityCount++;
            }
        }
        line = next;
    }
}

// O nome como se compara: sem espaço nas pontas, em minúsculas, e com "_" como
// espaço. O catálogo do cliente guarda "Andaluz_B" e é o controle de texto que
// desenha o "_" como espaço — então o título chega aqui com sublinhado, e quem
// escreve o GamePatch.txt escreve com espaço, como lê no jogo.
void NormalizeName(const char* in, char* out, size_t size) {
    strncpy_s(out, size, in != nullptr ? in : "", _TRUNCATE);
    for (char* c = out; *c != 0; c++) {
        if (*c == '_') {
            *c = ' ';
        } else if (*c >= 'A' && *c <= 'Z') {
            *c = static_cast<char>(*c - 'A' + 'a');
        }
    }
    Trim(out);
}

const Rarity* RarityOf(const char* title) {
    if (!g_raritiesLoaded) {
        LoadRarities();
    }
    char wanted[64];
    NormalizeName(title, wanted, sizeof(wanted));
    for (int i = 0; i < g_rarityCount; i++) {
        char have[64];
        NormalizeName(g_rarities[i].mount, have, sizeof(have));
        if (strcmp(have, wanted) == 0) {
            return &g_rarities[i];
        }
    }
    return nullptr;
}

// A linha "Montaria nível X" vai na primeira linha livre do corpo depois da
// última usada (a 14ª é o preço, e fica de fora). g_levelLine lembra onde ela
// foi escrita, para ser apagada quando o próximo tooltip não a quiser.
int g_levelLine = -1;
constexpr int kPriceLine = kTracked - 1;

void SetLineRaw(int i, const char* text, DWORD color) {
    void* control = TrackedControl(i);
    HookedVtable* hook = control != nullptr ? HookFor(control) : nullptr;
    if (hook == nullptr) {
        return;
    }
    hook->setText(control, nullptr, text, 0);
    if (color != 0) {
        hook->setColor(control, nullptr, color);
    }
}

void PlaceLevelLine(const Rarity* rarity) {
    // Apaga a linha do tooltip anterior, se este não a reescreveu.
    if (g_levelLine >= 0 && !g_lineUsed[g_levelLine]) {
        SetLineRaw(g_levelLine, "", 0);
    }
    g_levelLine = -1;
    if (rarity == nullptr || rarity->label[0] == 0) {
        return;
    }
    int last = 0;
    for (int i = 1; i < kPriceLine; i++) {
        if (g_lineUsed[i]) {
            last = i;
        }
    }
    int free = last + 1;
    if (free >= kPriceLine) {
        return;
    }
    char text[64];
    strcpy_s(text, sizeof(text), "Montaria n\xedvel ");
    strcat_s(text, sizeof(text), rarity->label);
    SetLineRaw(free, text, FamilyColor(rarity->family, 1.0f));
    g_levelLine = free;
}
// -----------------------------------------------------------------------------

// Outro tooltip está sendo montado nas mesmas linhas: o painel volta à cor do
// cliente, a borda deixa de ser desenhada e a linha de nível sai.
void ForgetMountTooltip() {
    if (!g_mountTooltip) {
        return;
    }
    g_mountTooltip = false;
    void* panel = TooltipPanel();
    if (panel != nullptr && g_panelRender != nullptr) {
        PanelColor(panel) = g_panelOriginalColor;
    }
    if (g_levelLine >= 0) {
        SetLineRaw(g_levelLine, "", 0);
        g_levelLine = -1;
    }
}

void __cdecl BeforeTooltip() {
    g_insideTooltip = true;
    if (!g_controlsHooked) {
        HookTooltipControls();
    }
    for (int i = 0; i < kTracked; i++) {
        g_wantedColor[i] = 0;
        g_clientColor[i] = 0;
        g_lineUsed[i] = false;
    }
    g_title[0] = 0;
    g_tooltipHasAbsorb = false;
    g_tooltipItem = -1;
    g_tooltipBuilt = false;
}

bool IsMountTooltip() {
    if (!g_slotHooked) {
        return g_tooltipHasAbsorb;
    }
    return g_tooltipItem >= kAdultMountLo && g_tooltipItem <= kAdultMountHi;
}

void __cdecl AfterTooltip() {
    g_insideTooltip = false;
    HookTooltipPanel();
    if (!g_tooltipBuilt) {
        return; // chamada que não montou tooltip: o que está na tela continua valendo
    }
    g_mountTooltip = IsMountTooltip();
    const Rarity* rarity = g_mountTooltip ? RarityOf(g_title) : nullptr;
    g_family = rarity != nullptr ? rarity->family : kDefaultFamily;
    void* panel = TooltipPanel();
    if (panel != nullptr && g_panelRender != nullptr) {
        PanelColor(panel) = g_mountTooltip ? kColorBackground : g_panelOriginalColor;
    }
    PlaceLevelLine(rarity);
    // A paleta é só do tooltip de montaria.
    if (!g_mountTooltip) {
        return;
    }
    g_applying = true;
    for (int i = 0; i < kTracked; i++) {
        void* control = TrackedControl(i);
        if (g_wantedColor[i] == 0 || control == nullptr || IsWarningColor(g_clientColor[i])) {
            continue;
        }
        HookedVtable* hook = HookFor(control);
        if (hook != nullptr) {
            hook->setColor(control, nullptr, g_wantedColor[i]);
        }
    }
    g_applying = false;
}

// Executa os oito bytes que o gancho cobriu e segue para o resto da função.
__declspec(naked) void TooltipTrampoline() {
    __asm {
        push ebp
        mov ebp, esp
        mov eax, 0x1498
        push 0x416A88 // 1ª instrução depois dos 8 bytes copiados; literal, porque em asm inline um nome viraria leitura de memória
        ret
    }
}

// A função original é __thiscall com três argumentos e termina em ret 0xC.
// O gancho repassa os três, chama a original pelo trampolim e pinta as linhas.
__declspec(naked) void TooltipHook() {
    __asm {
        push ecx
        call BeforeTooltip
        pop ecx
        push dword ptr [esp + 12]
        push dword ptr [esp + 12]
        push dword ptr [esp + 12]
        call TooltipTrampoline
        push eax
        call AfterTooltip
        pop eax
        ret 0xC
    }
}

// Anota o índice do item do slot e refaz as duas instruções que o desvio cobriu.
// ecx está livre aqui: o cliente o sobrescreve na instrução seguinte.
__declspec(naked) void SlotHook() {
    __asm {
        mov dword ptr [ebp - 0x8], eax
        mov g_tooltipItem, -1
        test eax, eax
        jz done
        mov ecx, dword ptr [eax + 0x670]
        test ecx, ecx
        jz done
        movsx ecx, word ptr [ecx]
        mov g_tooltipItem, ecx
    done:
        mov eax, dword ptr ds:[0x6F0AB0]
        push 0x416B56 // kSlotHookBack; literal pelo mesmo motivo do trampolim
        ret
    }
}

bool InstallSlotHook() {
    BYTE* target = reinterpret_cast<BYTE*>(kSlotHookAt);
    if (memcmp(target, kExpectedSlotBytes, sizeof(kExpectedSlotBytes)) != 0) {
        return false;
    }
    DWORD oldProtect = 0;
    if (!VirtualProtect(target, sizeof(kExpectedSlotBytes), PAGE_EXECUTE_READWRITE, &oldProtect)) {
        return false;
    }
    target[0] = 0xE9; // jmp rel32
    *reinterpret_cast<DWORD*>(target + 1) = reinterpret_cast<DWORD>(&SlotHook) - (kSlotHookAt + 5);
    target[5] = target[6] = target[7] = 0x90; // o resto das duas instruções cobertas
    VirtualProtect(target, sizeof(kExpectedSlotBytes), oldProtect, &oldProtect);
    FlushInstructionCache(GetCurrentProcess(), target, sizeof(kExpectedSlotBytes));
    return true;
}

bool InstallTooltipHook() {
    BYTE* target = reinterpret_cast<BYTE*>(kTooltipFunc);
    if (memcmp(target, kExpectedPrologue, sizeof(kExpectedPrologue)) != 0) {
        return false; // outra build: melhor sem cor do que com o jogo caindo
    }
    DWORD oldProtect = 0;
    if (!VirtualProtect(target, 5, PAGE_EXECUTE_READWRITE, &oldProtect)) {
        return false;
    }
    target[0] = 0xE9; // jmp rel32
    *reinterpret_cast<DWORD*>(target + 1) =
        reinterpret_cast<DWORD>(&TooltipHook) - (kTooltipFunc + 5);
    VirtualProtect(target, 5, oldProtect, &oldProtect);
    FlushInstructionCache(GetCurrentProcess(), target, 5);
    return true;
}

} // namespace

BOOL APIENTRY DllMain(HMODULE module, DWORD reason, LPVOID) {
    if (reason == DLL_PROCESS_ATTACH) {
        DisableThreadLibraryCalls(module);
        if (InstallTooltipHook()) {
            g_slotHooked = InstallSlotHook();
        }
    }
    return TRUE;
}
