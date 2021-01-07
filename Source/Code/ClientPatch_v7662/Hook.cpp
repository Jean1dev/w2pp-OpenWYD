/*
*   Copyright (C) {2015}  {Victor Klafke, Charles TheHouse}
*
*   This program is free software: you can redistribute it and/or modify
*   it under the terms of the GNU General Public License as published by
*   the Free Software Foundation, either version 3 of the License, or
*   (at your option) any later version.
*
*   This program is distributed in the hope that it will be useful,
*   but WITHOUT ANY WARRANTY; without even the implied warranty of
*   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
*   GNU General Public License for more details.
*
*   You should have received a copy of the GNU General Public License
*   along with this program.  If not, see [http://www.gnu.org/licenses/].
*
*   Contact at: victor.klafke@ecomp.ufsm.br
*/

#include "Main.h"

__declspec(naked) void NKD_FixMageMacro()
{
	__asm
	{
		MOV ECX, DWORD PTR SS : [EBP - 0x11C]
		// Diferente do Macro Continuo, então pula
		CMP DWORD PTR DS : [ECX + 0x8A3A0], 0x0
		JNZ lbl_JMP
		PUSH 0x04970F9
		RETN

		lbl_JMP :
		PUSH 0x04972E5
			RETN
	}
}

__declspec(naked) void NKD_Amount()
{/*0040CD91  |. 8B4D E0        MOV ECX,DWORD PTR SS:[EBP-20]            ; |
 0040CD94  |. 8B91 94000000  MOV EDX,DWORD PTR DS:[ECX+94]            ; |
 0040CD9A  |. 52             PUSH EDX                                 ; |Arg2
 NEste call fica a cor da descrição
 */
	__asm
	{
		MOV ECX, DWORD PTR SS : [EBP + 0Ch]
		MOVSX EAX, WORD PTR DS : [ECX]

		// ADICIONE mais aqui
		CMP EAX, 413
		JNZ lbl_continueNormal
	}

	static STRUCT_ITEM *item;
	static int* retn;
	__asm
	{
		LEA EAX, DWORD PTR SS : [EBP - 018h]
		MOV retn, EAX

		MOV ECX, DWORD PTR SS : [EBP + 0Ch]
		MOV DWORD PTR DS : [item], ECX
	}

	*retn = *(BYTE*)&item->stEffect[0].cEffect;

	__asm
	{
		MOV EDX, 0x40D7F8
		JMP EDX

		lbl_continueNormal :
		MOV ECX, DWORD PTR SS : [EBP + 0Ch]
			PUSH ECX

			MOV EAX, 0x539810
			CALL EAX

			MOV EDX, 0x40D7F2
			JMP EDX
	}
}

__declspec(naked) void NKD_AddVolatileMessageItem()
{
	__asm
	{
		CMP DWORD PTR SS : [EBP + 0xC], 0x15E2 // 0xFD1
		JNZ lblNext
		JMP lblSuccess
		lblNext :
		PUSH DWORD PTR SS : [EBP + 0xC]
			CALL AddVolatileMessageItem
			TEST EAX, EAX
			JE lblContinueExec
			lblSuccess :
		MOV EAX, 1
			lblContinueExec :
			PUSH 0x422017
			RETN 4
	}
}

__declspec(naked) void NKD_AddVolatileMessageBox()
{
	static char msg[128] = { 0 };

	__asm
	{
		CMP DWORD PTR SS : [EBP - 0x1AC], 0xD3
		JNZ lblNext
		PUSH 0x41FB3C
		RETN 4

		lblNext :
				LEA ECX, msg
				PUSH ECX
				MOV EAX, DWORD PTR SS : [EBP - 0x1B0]
				PUSH EAX
				CALL SetVolatileMessageBoxText
				TEST AL, AL
				JE lblContinueExec

				MOV EAX, DWORD PTR DS : [0x6F0AB0]
				MOV DWORD PTR SS : [EBP - 0x388], EAX
				PUSH    0 // 0
				PUSH	0x373 // 0x373
				LEA EAX, msg
				PUSH EAX
				MOV     ECX, DWORD PTR SS : [EBP - 0x388]
				MOV     ECX, DWORD PTR DS : [ECX + 0xCC]
				MOV     EDX, DWORD PTR SS : [EBP - 0x388]
				MOV     EAX, DWORD PTR DS : [EDX + 0xCC]
				MOV     EDX, DWORD PTR DS : [EAX]
				CALL    DWORD PTR DS : [EDX + 0x8C]
				MOV     EAX, DWORD PTR SS : [EBP + 0x8]
				SHL     EAX, 0x10
				OR      EAX, DWORD PTR SS : [EBP + 0xC]
				MOV     ECX, DWORD PTR SS : [EBP - 0x388]
				MOV     EDX, DWORD PTR DS : [ECX + 0xCC]
				MOV     DWORD PTR DS : [EDX + 0x1E8], EAX
				PUSH    1
				MOV     EAX, DWORD PTR SS : [EBP - 0x388]
				MOV     ECX, DWORD PTR DS : [EAX + 0xCC]
				MOV     EDX, DWORD PTR SS : [EBP - 0x388]
				MOV     EAX, DWORD PTR DS : [EDX + 0xCC]
				MOV     EDX, DWORD PTR DS : [EAX]
				CALL    DWORD PTR DS : [EDX + 0x60]
				PUSH 0x41FCFC
				RETN 4

				lblContinueExec :
								PUSH 0x41FBB2
								RETN 4
	}
}

__declspec(naked) void NKD_SendChat()
{
	__asm
	{
		MOV EAX, DWORD PTR SS : [EBP - 0x98]
		MOV EDX, DWORD PTR DS : [EAX]
		MOV ECX, DWORD PTR SS : [EBP - 0x98]
		CALL DWORD PTR DS : [EDX + 0x88]

		PUSH EAX
		CALL SendChat
		ADD ESP, 0x08

		TEST EAX, EAX
		JNZ chk_other_cmd

		MOV EAX, 0x01
		MOV ECX, 0x473794
		JMP ECX

		chk_other_cmd :

		MOV ECX, 0x467740
			JMP ECX
	}
}

__declspec(naked) void NKD_ReadMessage()
{
	__asm
	{
		MOV EAX, DWORD PTR SS : [EBP - 0x8]
		PUSH EAX
		MOV ECX, DWORD PTR SS : [EBP - 0x18]
		PUSH ECX
		CALL ReadMessage
		MOV DWORD PTR SS : [EBP - 0x18], EAX
		PUSH 0x4253CC
		RETN
	}
}

void LoadHooks()
{
	// Efeito de Buffs 6.13
	*(DWORD*)(0x0054A331 + 6) = 0;
	*(DWORD*)(0x0054A351 + 6) = 0;
	*(DWORD*)(0x00467651 + 6) = 0;

	// Força os graficos
	*(DWORD*)(0x427213 + 6) = 0;

	// Altera o salto das verificações de checksum para JMP afim de não verificar o checksum.
	*(BYTE*)0x53AC6A = 0xEB;
	*(BYTE*)0x53AD52 = 0xEB;
	*(BYTE*)0x53AE7E = 0xEB;

	JE_NEAR(0x04974C7, NKD_FixMageMacro);
	JE_NEAR(0x04974D7, NKD_FixMageMacro);

	// ReadMessage
	JGE_NEAR(0x4252D6, NKD_ReadMessage);

	// SendChat
	JMP_NEAR(0x4676C5, NKD_SendChat);

	// ITEM BOX
	JMP_NEAR(0x422007, NKD_AddVolatileMessageItem, 2); //421F1A
	JMP_NEAR(0x41FB30, NKD_AddVolatileMessageBox, 5);

	// Teste para SkillDelay
	for (int i = 0; i < 104; i++)
		*(int*)(0x11DA838 + (i * 96) + 48) = *(int*)(0x11DA838 + (i * 96) + 48) / 4;

	// Nome na janela
	strcpy((char*)0x0622CDC, "WYD CDK");
	}

/*void Closeclass(LPCSTR hackclass)
{
	DWORD pid;

	HWND hWnd = FindWindowA(hackclass, 0); // parametro 1 = classe , parametro = nome da janela
	GetWindowThreadProcessId(hWnd, &pid);

	if (hWnd) {
		ExitProcess(0);
	}

}

void ClassList() {
	Closeclass("nome da classe do hack");
}*/
